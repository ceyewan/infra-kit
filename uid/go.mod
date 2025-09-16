module github.com/ceyewan/infra-kit/uid

go 1.25.1

require (
	github.com/ceyewan/infra-kit/clog v0.0.0
	github.com/google/uuid v1.6.0
	github.com/stretchr/testify v1.8.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/ceyewan/infra-kit/clog => ../clog
