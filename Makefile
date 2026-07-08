PLUGIN_NAME := packer-plugin-verda
MODULE := $(shell go list -m)
VERSION_PKG := $(MODULE)/version
VERSION ?= 0.1.0
PLUGIN_SOURCE := github.com/thevilledev/verda
COUNT ?= 1
TEST ?= ./...
HASHICORP_PACKER_PLUGIN_SDK_VERSION ?= $(shell go list -m github.com/hashicorp/packer-plugin-sdk | cut -d " " -f2)
GOLANGCI_LINT_VERSION ?= v2.11.4
TOOLS_BIN := $(CURDIR)/.tools/bin
PACKER_SDC_STAMP := $(TOOLS_BIN)/.packer-sdc-$(HASHICORP_PACKER_PLUGIN_SDK_VERSION)
GOLANGCI_LINT_STAMP := $(TOOLS_BIN)/.golangci-lint-$(GOLANGCI_LINT_VERSION)
DOCS_RENDER_DIR ?= .docs
DOCS_SITE_DIR ?= .site
WEB_DOCS_DIR ?= .web-docs

.PHONY: build dev test testacc lint fmt check-fmt tidy tidy-check install-packer-sdc install-golangci-lint generate renderdocs webdocs docs-site check-generate plugin-check plugin-install snapshot release clean

build:
	go build -trimpath -ldflags="-X $(VERSION_PKG).Version=$(VERSION)" -o $(PLUGIN_NAME) .

dev:
	go build -trimpath -ldflags="-X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).VersionPrerelease=dev" -o $(PLUGIN_NAME) .

test:
	go test -race -count $(COUNT) $(TEST) -timeout=3m

testacc: dev
	PACKER_ACC=1 go test -count $(COUNT) -v $(TEST) -timeout=120m

lint: install-golangci-lint
	PATH="$(TOOLS_BIN):$$PATH" golangci-lint run ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

check-fmt:
	test -z "$$(gofmt -l .)"

tidy:
	go mod tidy

tidy-check:
	go mod tidy
	git diff --exit-code go.mod go.sum

$(PACKER_SDC_STAMP):
	mkdir -p $(TOOLS_BIN)
	GOBIN=$(TOOLS_BIN) go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@$(HASHICORP_PACKER_PLUGIN_SDK_VERSION)
	touch $(PACKER_SDC_STAMP)

install-packer-sdc: $(PACKER_SDC_STAMP)

$(GOLANGCI_LINT_STAMP):
	mkdir -p $(TOOLS_BIN)
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	touch $(GOLANGCI_LINT_STAMP)

install-golangci-lint: $(GOLANGCI_LINT_STAMP)

generate: install-packer-sdc
	PATH="$(TOOLS_BIN):$$PATH" go generate ./...
	$(MAKE) webdocs

renderdocs: generate
	rm -rf "$(DOCS_RENDER_DIR)"
	PATH="$(TOOLS_BIN):$$PATH" packer-sdc renderdocs -src docs -partials docs-partials/ -dst "$(DOCS_RENDER_DIR)/"

webdocs: install-packer-sdc
	rm -rf "$(DOCS_RENDER_DIR)"
	PATH="$(TOOLS_BIN):$$PATH" packer-sdc renderdocs -src docs -partials docs-partials/ -dst "$(DOCS_RENDER_DIR)/"
	rm -rf "$(WEB_DOCS_DIR)/README.md" "$(WEB_DOCS_DIR)/components"
	"./$(WEB_DOCS_DIR)/scripts/compile-to-webdocs.sh" "." "$(DOCS_RENDER_DIR)" "$(WEB_DOCS_DIR)" "thevilledev"

docs-site: renderdocs
	rm -rf "$(DOCS_SITE_DIR)"
	mkdir -p "$(DOCS_SITE_DIR)"
	cp -R "$(DOCS_RENDER_DIR)/." "$(DOCS_SITE_DIR)/"
	find "$(DOCS_SITE_DIR)" -name '*.mdx' -exec sh -c 'mv "$$1" "$${1%.mdx}.md"' _ {} \;

check-generate: generate
	git diff --exit-code

plugin-check: install-packer-sdc build
	PATH="$(TOOLS_BIN):$$PATH" packer-sdc plugin-check $(PLUGIN_NAME)

plugin-install: build
	packer plugins install --path ./$(PLUGIN_NAME) $(PLUGIN_SOURCE)

snapshot:
	API_VERSION="$$(go run . describe | sed -n 's/.*"api_version":"\([^"]*\)".*/\1/p')" goreleaser release --snapshot --clean --skip=sign

release:
	API_VERSION="$$(go run . describe | sed -n 's/.*"api_version":"\([^"]*\)".*/\1/p')" goreleaser release --clean

clean:
	rm -f $(PLUGIN_NAME)
	rm -rf "$(DOCS_RENDER_DIR)" "$(DOCS_SITE_DIR)" dist
