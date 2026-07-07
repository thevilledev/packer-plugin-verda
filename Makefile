PLUGIN_NAME := packer-plugin-verda
MODULE := $(shell go list -m)
VERSION_PKG := $(MODULE)/version
VERSION ?= 0.1.0
PLUGIN_SOURCE := github.com/thevilledev/verda

.PHONY: build dev test lint fmt tidy plugin-check snapshot release clean

build:
	go build -trimpath -ldflags="-X $(VERSION_PKG).Version=$(VERSION)" -o $(PLUGIN_NAME) .

dev:
	go build -trimpath -ldflags="-X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).VersionPrerelease=dev" -o $(PLUGIN_NAME) .

test:
	go test -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	go mod tidy

plugin-check: build
	packer plugins install --path ./$(PLUGIN_NAME) $(PLUGIN_SOURCE)

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean

clean:
	rm -f $(PLUGIN_NAME)
