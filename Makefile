SHELL=/bin/bash -o pipefail

# ROOT_DIR holds the absolute path to the root of the dklb repository.
ROOT_DIR := $(shell git rev-parse --show-toplevel)

# VERSION holds the version of dklb being built.
VERSION ?= $(shell git describe --always --dirty=-dev)

# Ensure go mod is always used.
export GO111MODULE := on

include make/platform.mk
include make/ci.mk
include make/dcos-terraform.mk
include make/mke.mk
include make/edgelb.mk

.DEFAULT_GOAL := build

# build builds the dklb binary for the specified architecture (defaults to "amd64") and operating system (defaults to "linux").
.PHONY: build
build: GOARCH ?= amd64
build: GOFLAGS ?= ""
build: GOOS ?= linux
build: LDFLAGS ?= -s -w
build: gitauth
	@CGO_ENABLED=0 GOARCH=$(GOARCH) GOFLAGS=$(GOFLAGS) GOOS=$(GOOS) go build \
		-ldflags="$(LDFLAGS) -X github.com/mesosphere/dklb/pkg/version.Version=$(VERSION)" \
		-o $(ROOT_DIR)/build/dklb \
		-v \
		$(ROOT_DIR)/cmd/main.go

# docker builds a Docker image of dklb suitable for distribution.
.PHONY: docker
docker: IMG ?= mesosphere/dklb
docker: TAG ?= $(VERSION)
docker:
# We depend on "github.com/mesosphere/dcos-edge-lb", which can't be pulled from inside "docker build"
# without specifying a GITHUB_TOKEN that can be used to pull private Mesosphere repositories.
ifndef GITHUB_TOKEN
	$(error Please specify GITHUB_TOKEN that can be used to pull private Mesosphere repositories)
endif
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GITHUB_TOKEN=$(GITHUB_TOKEN) \
		--tag "$(IMG):$(TAG)" $(ROOT_DIR)

# skaffold deploys dklb to the Kubernetes repository targeted by the current context using skaffold.
.PHONY: skaffold
skaffold: MODE ?= dev
skaffold: gitauth dockerauth service-account
	@if [[ ! "$(MODE)" == "delete" ]]; then \
		GOOS=linux GOARCH=amd64 $(MAKE) -C $(ROOT_DIR) build; \
	fi
	@skaffold $(MODE) -f $(ROOT_DIR)/hack/skaffold/dklb/skaffold.yaml

# service account creates the DC/OS service account and sets up the Kubernetes secret
.PHONY: service-account
service-account:
	./hack/service-account.sh

# test.e2e runs the end-to-end test suite.
.PHONY: test.e2e
test.e2e: AWS_PUBLIC_SUBNET_IDS ?=
test.e2e: FOCUS ?= .*
test.e2e: KUBECONFIG ?= $(HOME)/.kube/config
test.e2e: LOG_LEVEL := info
test.e2e: TIMEOUT := 1800s
test.e2e: gitauth
	@go test -tags e2e $(ROOT_DIR)/test/e2e \
		-ginkgo.focus="$(FOCUS)" \
		-ginkgo.v \
		-test.timeout="$(TIMEOUT)" \
		-test.v \
		$(foreach subnet,$(AWS_PUBLIC_SUBNET_IDS),-aws-public-subnet-id="$(subnet)") \
		-edgelb-bearer-token="$$(dcos config show core.dcos_acs_token)" \
		-edgelb-host="$$(dcos config show core.dcos_url)" \
		-edgelb-insecure-skip-tls-verify \
		-edgelb-path="/service/edgelb" \
		-edgelb-scheme="https" \
		-kubeconfig="$(KUBECONFIG)" \
		-log-level="$(LOG_LEVEL)"

# test.unit runs the unit test suite.
.PHONY: test.unit
test.unit: gitauth
	@go test -race -v $(ROOT_DIR)/...

gitauth:
ifdef GITHUB_TOKEN
	@git config --global url."https://$(GITHUB_TOKEN):x-oauth-basic@github.com/mesosphere".insteadOf "https://github.com/mesosphere"
endif
