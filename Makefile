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

# Testcontainers (podman rootless) environment for integration/db tests.
TC_ENV := DOCKER_HOST=unix:///run/user/$(shell id -u)/podman/podman.sock \
          TESTCONTAINERS_RYUK_DISABLED=true

.PHONY: build test test-unit test-db test-integration lint clean

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/kb ./cmd/kb

test: test-unit test-db test-integration

test-unit:
	ginkgo --label-filter='!db && !integration' ./...

test-db:
	$(TC_ENV) ginkgo --label-filter='db' ./...

test-integration:
	$(TC_ENV) ginkgo --label-filter='integration' ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
