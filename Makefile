.DEFAULT_GOAL := build

PKG ?= "github.com/mesosphere/dklb"

# This version-strategy uses git tags to set the version string
VERSION ?= $(shell git describe --tags --always --dirty)

build: lint
	@CGO_ENABLED=0 go build \
		-tags=netgo -installsuffix=netgo \
		-ldflags "-d -s -w -X ${PKG}/pkg/version.Version=${VERSION}" \
		-o dklb cmd/dklb/main.go

clean:
	@rm -f dklb

lint:
	@golint ./cmd/... ./pkg/...

test: build
	@go test ./...