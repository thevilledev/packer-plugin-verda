PLUGIN_NAME := packer-plugin-verda
MODULE := $(shell go list -m)
VERSION_PKG := $(MODULE)/version
VERSION ?= 0.1.0
PLUGIN_SOURCE := github.com/thevilledev/verda
HASHICORP_PACKER_PLUGIN_SDK_VERSION ?= $(shell go list -m github.com/hashicorp/packer-plugin-sdk | cut -d " " -f2)
TOOLS_BIN := $(CURDIR)/.tools/bin
PACKER_SDC_STAMP := $(TOOLS_BIN)/.packer-sdc-$(HASHICORP_PACKER_PLUGIN_SDK_VERSION)

.PHONY: build dev test lint fmt tidy install-packer-sdc generate check-generate plugin-check snapshot release clean

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

$(PACKER_SDC_STAMP):
	GOBIN=$(TOOLS_BIN) go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@$(HASHICORP_PACKER_PLUGIN_SDK_VERSION)
	touch $(PACKER_SDC_STAMP)

install-packer-sdc: $(PACKER_SDC_STAMP)

generate: install-packer-sdc
	PATH="$(TOOLS_BIN):$$PATH" go generate ./...

check-generate: generate
	git diff --exit-code

plugin-check: build
	packer plugins install --path ./$(PLUGIN_NAME) $(PLUGIN_SOURCE)

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean

clean:
	rm -f $(PLUGIN_NAME)
