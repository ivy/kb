MODULE   := github.com/ivy/kb
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_REF  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TAG ?= local
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -s -w \
  -X '$(MODULE)/internal/cli.version=$(VERSION)' \
  -X '$(MODULE)/internal/cli.gitRef=$(GIT_REF)' \
  -X '$(MODULE)/internal/cli.buildTag=$(BUILD_TAG)' \
  -X '$(MODULE)/internal/cli.buildDate=$(BUILD_DATE)'

.PHONY: build test test-unit lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/kb ./cmd/kb

test:
	ginkgo ./...

test-unit:
	ginkgo --label-filter='!db' ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
