.PHONY: help build test clean lint format release

# 默认目标
help:
	@echo "Available commands:"
	@echo "  build     - Build all components"
	@echo "  test      - Run all tests"
	@echo "  lint      - Run linter on all components"
	@echo "  format    - Format all Go code"
	@echo "  clean     - Clean build artifacts"
	@echo "  release   - Create release (requires gh CLI)"

# 构建所有组件
build:
	@for dir in clog uid coord cache db mq ratelimit once breaker es metrics; do \
		echo "Building $$dir..."; \
		(cd $$dir && go build ./...); \
	done

# 运行所有测试
test:
	@for dir in clog uid coord cache db mq ratelimit once breaker es metrics; do \
		echo "Testing $$dir..."; \
		(cd $$dir && go test -v ./...); \
	done

# 运行代码检查
lint:
	@for dir in clog uid coord cache db mq ratelimit once breaker es metrics; do \
		echo "Linting $$dir..."; \
		(cd $$dir && golangci-lint run); \
	done

# 格式化代码
format:
	@for dir in clog uid coord cache db mq ratelimit once breaker es metrics; do \
		echo "Formatting $$dir..."; \
		(cd $$dir && go fmt ./...); \
	done

# 清理构建产物
clean:
	@for dir in clog uid coord cache db mq ratelimit once breaker es metrics; do \
		echo "Cleaning $$dir..."; \
		(cd $$dir && go clean ./...); \
	done
	rm -rf */*.out

# 创建发布
release: clean test lint
	@echo "Creating release..."
	gh release create $(VERSION) --generate-notes

# 初始化开发环境
init:
	@echo "Initializing development environment..."
	go install golangci-lint@latest
	go install github.com/goreleaser/goreleaser@latest
	go mod tidy
	go work sync

# 更新依赖
deps:
	@for dir in clog uid coord cache db mq ratelimit once breaker es metrics; do \
		echo "Updating dependencies for $$dir..."; \
		(cd $$dir && go mod tidy && go mod download); \
	done
	go work sync
