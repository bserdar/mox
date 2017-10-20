# Set an output prefix, which is the local directory if not specified
PREFIX?=$(shell pwd)

.PHONY: all build binaries clean fmt lint test test-full vet
.DEFAULT: all
all: fmt vet lint build test binaries
quick: fmt lint build binaries

# Go files
GOFILES=$(shell find . -type f -name '*.go') $(shell find cmd -type f -name '*.go')

# Package list
PKGS=$(shell go list  ./...| grep -v /vendor/)

# Resolving binary dependencies for specific targets
GOLINT=$(shell which golint || echo '')

${PREFIX}/bin/mox: $(GOFILES)
	@echo "+ $@"
	@go build  -o $@ ./cmd/mox

vet:
	@echo "+ $@"
	@go vet  $(PKGS)

fmt:
	@echo "+ $@"
	@test -z "$$(gofmt -s -l . 2>&1 | grep -v ^vendor/ | tee /dev/stderr)" || \
		(echo >&2 "+ please format Go code with 'gofmt -s'" && false)

lint:
	@echo "+ $@"
	$(if $(GOLINT), , \
		$(error Please install golint: `go get -u github.com/golang/lint/golint`))
	@test -z "$$($(GOLINT) ./... 2>&1 | grep -v ^vendor/ | tee /dev/stderr)"

build: fmt vet lint
	@echo "+ $@"
	@go build  $(PKGS)

test:
	@echo "+ $@"
	@echo $(MONGO)
	go test -test.short  $(PKGS) 

test-full:
	@echo "+ $@"
	@go test  $(PKGS)

binaries: ${PREFIX}/bin/mox
	@echo "+ $@"

clean:
	@echo "+ $@"
	@rm -rf "${PREFIX}/bin/mox"

docker: binaries
	docker build -t mox:latest .
