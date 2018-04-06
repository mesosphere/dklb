.DEFAULT_GOAL := build

PKG ?= "github.com/mesosphere/dklb"

# This version-strategy uses git tags to set the version string
VERSION ?= $(shell git describe --tags --always --dirty)

build:
	@go build \
		-ldflags "-w -extldflags=-static -X ${PKG}/pkg/version.Version=${VERSION}" \
		-o dklb cmd/dklb/main.go

clean:
	@rm -f dklb

test: build
	@go test ./...