.PHONY: build test fmt vet

build:
	go build -trimpath -ldflags="-s -w" -o bin/releasepanel-agent ./cmd/agent

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...
