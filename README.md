go get go.uber.org/zap # For logging
go get github.com/spf13/viper # For config
go get github.com/openai/openai-go/v2 # For AI/LLM
go get github.com/spf13/cobra # For CLI
go get github.com/fatih/color # For colored output

Installation
//folder stracture
//linkes between folders
//soild principles
use go formatting

Hexagonal Architecture (Ports & Adapters) - separating core business logic (internal/agent, internal/tools) from external interfaces (cmd/) and infrastructure (internal/core, internal/storage).

tools → agent → cmd
│ │ │
│ │ └── print / handle
│ └── Wrap
└── Root Error

1. **Logger** -  log messages at different levels
2. **Config** -  load settings from files and env vars
3. **Errors** -  create rich, contextual errors

### Files till now:

```
internal/
├── core/
│   ├── logger/
│   │   ├── logger.go
│   │   └── logger_test.go
│   └── config/
│       ├── config.go
│       └── config_test.go
└── errors/
    └── types/
        ├── errors.go
        └── errors_test.go

configs/
└── dev/
    └── config.yaml

cmd/
├── test-logger/
│   └── main.go
├── test-config/
│   └── main.go
└── test-all/
    └── main.go
```

### testing commands

# Run all tests

go test ./...
