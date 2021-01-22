GO      ?= GO111MODULE=on go
GOBUILD  = $(GO) build
GORUN    = $(GO) run
GOMOD    = $(GO) mod
GOCLEAN  = $(GO) clean
GOTEST   = $(GO) test
GOFMT    = $(GO) fmt
GOVET    = $(GO) vet
GOLIST   = $(GO) list
GOGET    = $(GO) get

TARGET_NAME = " ---> [$@]"

.PHONY: help deps fmt vet tests
all: help
help: Makefile
	@echo
	@echo 'Usage: make <TARGETS> ... <OPTIONS>'
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo
	@echo 'By default print this message.'
	@echo


## deps: Download dependencies.
deps:
	@echo $(TARGET_NAME)
	$(GOMOD) tidy
	$(GOMOD) download

## fmt: gofmt (reformat) package sources
fmt:
	@echo $(TARGET_NAME)
	@$(GOFMT) $(PKGS)

## vet: report likely mistakes in packages
vet: fmt
	@echo $(TARGET_NAME)
	@$(GOVET) $(PKGS)

## revive: run static analisator revive
revive:
	@echo $(TARGET_NAME)
ifeq (, $(shell which revive))
	@echo "install revive..."
	$(GOGET) -u github.com/mgechev/revive
endif
	@revive ./...

## staticcheck: run static analisator staticcheck
staticcheck:
	@echo $(TARGET_NAME)
ifeq (, $(shell which staticcheck))
	@echo "install staticcheck..."
	$(GOGET) -u honnef.co/go/tools/cmd/staticcheck
endif
	@staticcheck ./...

## golangci-lint: run static analisator golangci-lint if installed
golangci-lint:
	@echo $(TARGET_NAME)
ifeq (, $(shell which golangci-lint))
	@echo "golangci-lint not installed"
	@echo "to install: curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $(go env GOPATH)/bin v1.21.0"
	@echo "on alpine:  wget -O - -q https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s v1.21.0"
	@echo "on macosx:  brew install golangci/tap/golangci-lint"
else
	@-golangci-lint run ./...
endif

release:
	@echo $(TARGET_NAME)
ifeq (, $(shell which gorelease))
	@echo "install gorelease..."
	$(GOGET) -u golang.org/x/exp/cmd/gorelease
endif
	@gorelease

## tests: Running "go test" on sources packages.
tests: fmt vet revive
	@echo $(TARGET_NAME)
	@$(GOTEST) -count=1 ./... -coverprofile=coverage.txt -covermode=atomic