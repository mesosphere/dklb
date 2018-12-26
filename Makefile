# ROOT_DIR holds the absolute path to the root of the dklb repository.
ROOT_DIR := $(shell git rev-parse --show-toplevel)

# VERSION holds the version of dklb being built.
VERSION ?= $(shell git describe --always --dirty=-dev)

# build builds the dklb binary for the specified architecture (defaults to "amd64") and operating system (defaults to "linux").
.PHONY: build
build: GOARCH ?= amd64
build: GOOS ?= linux
build: LDFLAGS ?= -d -s -w
build: mod
build:
	@GOARCH=$(GOARCH) GOOS=$(GOOS) go build \
		-ldflags="$(LDFLAGS) -X github.com/mesosphere/dklb/pkg/version.Version=$(VERSION)" \
		-o $(ROOT_DIR)/build/dklb \
		-v \
		cmd/main.go

# mod downloads dependencies to the local cache.
.PHONY: mod
mod:
	@go mod download

# skaffold deploys dklb to the Kubernetes repository targeted by the current context using skaffold.
.PHONY: skaffold
skaffold: MODE ?= dev
skaffold:
	@if [[ ! "$(MODE)" == "delete" ]]; then \
		GOOS=linux GOARCH=amd64 $(MAKE) -C $(ROOT_DIR) build; \
	fi
	@skaffold $(MODE) -f $(ROOT_DIR)/hack/skaffold/dklb/skaffold.yaml

# test.e2e runs the end-to-end test suite.
.PHONY: test.e2e
test.e2e: DCOS_PUBLIC_AGENT_IP :=
test.e2e: KUBECONFIG ?= $(HOME)/.kube/config
test.e2e:
	@if [[ "$(DCOS_PUBLIC_AGENT_IP)" == "" ]]; then \
		echo "error: DCOS_PUBLIC_AGENT_IP must be set"; \
		exit 1; \
	fi
	@go test -tags e2e $(ROOT_DIR)/test/e2e \
		-ginkgo.v \
		-test.v \
		-dcos-public-agent-ip $(DCOS_PUBLIC_AGENT_IP) \
		-edgelb-bearer-token "$$(dcos config show core.dcos_acs_token)" \
		-edgelb-host "$$(dcos config show core.dcos_url)" \
		-edgelb-insecure-skip-tls-verify \
		-edgelb-path "/service/edgelb" \
		-edgelb-scheme https \
		-kubeconfig $(KUBECONFIG)

# test.unit runs the unit test suite.
.PHONY: test.unit
test.unit:
	@go test -v $(ROOT_DIR)/...
