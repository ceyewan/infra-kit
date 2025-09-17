package lockimpl

import (
	"context"
	"path"
	"time"

	"github.com/ceyewan/infra-kit/clog"
	"github.com/ceyewan/infra-kit/coord/internal/client"
	"github.com/ceyewan/infra-kit/coord/lock"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// EtcdLockFactory 是用于创建基于 etcd 的分布式锁的工厂。
// 实现了 lock.DistributedLock 接口。
type EtcdLockFactory struct {
	client *client.EtcdClient // etcd 客户端
	prefix string             // 锁的前缀
	logger clog.Logger        // 日志记录器
}

// NewEtcdLockFactory 创建一个 etcd 分布式锁工厂
func NewEtcdLockFactory(c *client.EtcdClient, prefix string, logger clog.Logger) *EtcdLockFactory {
	if prefix == "" {
		prefix = "/locks"
	}
	if logger == nil {
		logger = clog.Namespace("coordination.lock")
	}
	return &EtcdLockFactory{
		client: c,
		prefix: prefix,
		logger: logger,
	}
}

// Acquire 获取一个新锁，阻塞直到锁被获取或 context 被取消
func (f *EtcdLockFactory) Acquire(ctx context.Context, key string, ttl time.Duration) (lock.Lock, error) {
	return f.acquire(ctx, key, ttl, true)
}

// TryAcquire 尝试获取新锁，不阻塞
func (f *EtcdLockFactory) TryAcquire(ctx context.Context, key string, ttl time.Duration) (lock.Lock, error) {
	return f.acquire(ctx, key, ttl, false)
}

// acquire 内部实现，支持阻塞和非阻塞获取锁
func (f *EtcdLockFactory) acquire(ctx context.Context, key string, ttl time.Duration, blocking bool) (lock.Lock, error) {
	if key == "" {
		return nil, client.NewError(client.ErrCodeValidation, "lock key cannot be empty", nil)
	}
	if ttl <= 0 {
		return nil, client.NewError(client.ErrCodeValidation, "lock ttl must be positive", nil)
	}

	// 创建会话，包含租约并自动续约。锁释放时关闭会话。
	session, err := concurrency.NewSession(f.client.Client(), concurrency.WithTTL(int(ttl.Seconds())))
	if err != nil {
		return nil, client.NewError(client.ErrCodeConnection, "failed to create etcd session", err)
	}

	lockKey := path.Join(f.prefix, key)
	mutex := concurrency.NewMutex(session, lockKey)

	f.logger.Debug("尝试获取锁",
		clog.String("key", lockKey),
		clog.Int64("lease", int64(session.Lease())),
		clog.Bool("blocking", blocking))

	var lockErr error
	if blocking {
		// 阻塞直到获取锁或 context 被取消
		lockErr = mutex.Lock(ctx)
	} else {
		// 非阻塞尝试获取锁，立即返回
		lockErr = mutex.TryLock(ctx)
	}

	if lockErr != nil {
		_ = session.Close() // 尝试关闭会话，释放资源
		if lockErr == concurrency.ErrLocked {
			return nil, client.NewError(client.ErrCodeConflict, "lock is already held", lockErr)
		}
		return nil, client.NewError(client.ErrCodeConnection, "failed to acquire lock", lockErr)
	}

	f.logger.Info("锁获取成功",
		clog.String("key", lockKey),
		clog.Int64("lease", int64(session.Lease())))

	return &EtcdLock{
		session: session,
		mutex:   mutex,
		client:  f.client,
		logger:  f.logger,
	}, nil
}

// EtcdLock 表示已持有的分布式锁
type EtcdLock struct {
	session *concurrency.Session // etcd 会话，管理租约
	mutex   *concurrency.Mutex   // etcd 互斥锁
	client  *client.EtcdClient   // etcd 客户端
	logger  clog.Logger          // 日志记录器
}

// Unlock 释放锁
func (l *EtcdLock) Unlock(ctx context.Context) error {
	// 在所有操作之前缓存 key 和 lease，防止 session 关闭后无法获取
	key := l.mutex.Key()
	leaseID := l.session.Lease()

	l.logger.Debug("准备释放锁",
		clog.String("key", key),
		clog.Int64("lease", int64(leaseID)))

	// 先解锁互斥锁
	if err := l.mutex.Unlock(ctx); err != nil {
		// 即使解锁失败，也必须关闭会话以释放租约
		_ = l.session.Close()
		return client.NewError(client.ErrCodeConnection, "failed to unlock mutex", err)
	}

	// 关闭会话，撤销租约，最终释放锁
	if err := l.session.Close(); err != nil {
		return client.NewError(client.ErrCodeConnection, "failed to close session", err)
	}

	// 使用缓存的 key 进行日志记录
	l.logger.Info("锁释放成功", clog.String("key", key))
	return nil
}

// TTL 返回锁租约的剩余存活时间
func (l *EtcdLock) TTL(ctx context.Context) (time.Duration, error) {
	// 通过会话获取租约 ID
	resp, err := l.client.Client().TimeToLive(ctx, l.session.Lease())
	if err != nil {
		return 0, client.NewError(client.ErrCodeConnection, "failed to get lock TTL", err)
	}

	if resp.TTL <= 0 {
		// 如果租约刚好过期会出现这种情况
		return 0, client.NewError(client.ErrCodeNotFound, "lock has expired", nil)
	}

	return time.Duration(resp.TTL) * time.Second, nil
}

// Key 返回锁在 etcd 中的完整键路径
func (l *EtcdLock) Key() string {
	return l.mutex.Key()
}

// Renew 手动续约锁的TTL，返回是否成功
func (l *EtcdLock) Renew(ctx context.Context) (bool, error) {
	// 检查会话是否仍然有效
	select {
	case <-l.session.Done():
		// 会话已关闭，锁已过期
		l.logger.Warn("会话已关闭，无法续约", clog.String("key", l.mutex.Key()))
		return false, lock.ErrLockExpired
	default:
		// 会话仍然有效
	}

	// 尝试续约租约 - 使用 KeepAliveOnce 进行单次续约
	resp, err := l.client.Client().KeepAliveOnce(ctx, l.session.Lease())
	if err != nil {
		l.logger.Error("租约续约失败", clog.String("key", l.mutex.Key()), clog.String("error", err.Error()))
		return false, client.NewError(client.ErrCodeConnection, "failed to renew lease", err)
	}

	if resp == nil || resp.TTL <= 0 {
		// 租约已过期
		return false, lock.ErrLockExpired
	}

	l.logger.Debug("租约续约成功", clog.String("key", l.mutex.Key()), clog.Int64("ttl", int64(resp.TTL)))
	return true, nil
}

// IsExpired 检查锁是否已过期
func (l *EtcdLock) IsExpired(ctx context.Context) (bool, error) {
	// 首先检查会话状态
	select {
	case <-l.session.Done():
		// 会话已关闭，锁已过期
		return true, lock.ErrLockExpired
	default:
		// 会话仍然有效，继续检查租约
	}

	// 检查租约的TTL
	ttl, err := l.TTL(ctx)
	if err != nil {
		return false, err
	}

	return ttl <= 0, nil
}
