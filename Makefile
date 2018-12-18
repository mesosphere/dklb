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
skaffold: KUBERNETES_CLUSTER_FRAMEWORK_NAME ?= kubernetes-cluster
skaffold: MODE ?= dev
skaffold: CONFIG_MAP_NAME := dklb
skaffold: NAMESPACE := kube-system
skaffold:
	@kubectl -n $(NAMESPACE) delete configmap $(CONFIG_MAP_NAME) --ignore-not-found > /dev/null 2>&1
	@if [[ ! "$(MODE)" == "delete" ]]; then \
		GOOS=linux GOARCH=amd64 $(MAKE) -C $(ROOT_DIR) build; \
		kubectl -n $(NAMESPACE) create configmap $(CONFIG_MAP_NAME) --from-literal KUBERNETES_CLUSTER_FRAMEWORK_NAME=$(KUBERNETES_CLUSTER_FRAMEWORK_NAME); \
	fi
	@skaffold $(MODE) -f $(ROOT_DIR)/hack/skaffold/dklb/skaffold.yaml -n $(NAMESPACE)

# test.unit runs the unit test suite.
test.unit:
	@go test -v ./...
