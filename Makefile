VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/hungtrd/lazytodo/internal/cli.version=$(VERSION)

run:
	go run ./cmd/lazytodo

ui:
	go run ./cmd/lazytodo --ui

build:
	go build -ldflags "$(LDFLAGS)" -o lazytodo ./cmd/lazytodo

test:
	go test ./... -cover

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/lazytodo
