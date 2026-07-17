run:
	go run ./cmd/lazytodo

ui:
	go run ./cmd/lazytodo --ui

build:
	go build -o lazytodo ./cmd/lazytodo

test:
	go test ./... -cover

install:
	go install ./cmd/lazytodo
