PLUGIN_NAME := packer-plugin-verda
MODULE := $(shell go list -m)
VERSION_PKG := $(MODULE)/version

.PHONY: build test lint fmt tidy plugin-check clean

build:
	go build -trimpath -ldflags="-X $(VERSION_PKG).VersionPrerelease=dev" -o $(PLUGIN_NAME) .

test:
	go test -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

tidy:
	go mod tidy

plugin-check: build
	packer plugins install --path ./$(PLUGIN_NAME) github.com/verda-cloud/verda

clean:
	rm -f $(PLUGIN_NAME)
