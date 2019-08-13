# kubernetes Makefile TARGETS

# KUBERNETES_SERVICE_NAME sets a default service name for kubernetes
KUBERNETES_SERVICE_NAME := kubernetes

# KUBERNETES_SERVICE_ACCOUNT_NAME sets a default service account name for kubernetes
KUBERNETES_SERVICE_ACCOUNT_NAME := kubernetes

# KUBERNETES_PACKAGE_NAME sets a default package name for kubernetes
KUBERNETES_PACKAGE_NAME := kubernetes

# KUBERNETES_CLUSTER_SERVICE_NAME sets a default service name for kubernetes
KUBERNETES_CLUSTER_SERVICE_NAME := kubernetes-cluster

# SERVICE_NAME_DOMAIN removes any forward slashes
KUBERNETES_CLUSTER_SERVICE_NAME_DOMAIN := $(shell echo $(KUBERNETES_CLUSTER_SERVICE_NAME) | sed 's/\///g')

# KUBERNETES_CLUSTER_SERVICE_ACCOUNT_NAME sets a default service account name for kubernetes
KUBERNETES_CLUSTER_SERVICE_ACCOUNT_NAME := kubernetes-cluster

# KUBERNETES_CLUSTER_PACKAGE_NAME sets a default package name for kubernetes cluster
KUBERNETES_CLUSTER_PACKAGE_NAME := kubernetes-cluster

kubernetes.package.install: dcos.setup-security.kubernetes
ifeq ($(SECURITY), strict)
	@dcos package install --yes $(KUBERNETES_PACKAGE_NAME) --options=$(CURDIR)/hack/mke/kubernetes-strict-package-options.json
else
	@dcos package install --yes $(KUBERNETES_PACKAGE_NAME)
endif
	$(MAKE) package.kubernetes.await-healthy

kubernetes.package.uninstall:
	@dcos package uninstall --yes $(KUBERNETES_PACKAGE_NAME)
	@dcos package repo remove $(KUBERNETES_REPO_NAME)

# NOTE: Kubernetes package normally takes approx. 1-2 minutes to be running and healthy.
package.kubernetes.await-healthy:
	@$(call retry,\
		dcos kubernetes manager plan status deploy --json 2> /dev/null | jq -e '.status == "COMPLETE"' &> /dev/null,\
		8,\
		15,\
		Kubernetes installation isn't ready yet. Retrying in 15 seconds...)
	@echo "Kubernetes package is healthy!"

.PHONY: dcos.setup-security
dcos.setup-security: dcos.setup-security.kubernetes dcos.setup-security.kubernetes-cluster

dcos.setup-security.kubernetes-cluster: SECURITY_SERVICE_NAME = $(KUBERNETES_CLUSTER_SERVICE_NAME)
dcos.setup-security.kubernetes-cluster: SECURITY_SERVICE_ACCOUNT_NAME = $(KUBERNETES_CLUSTER_SERVICE_ACCOUNT_NAME)
dcos.setup-security.kubernetes: SECURITY_SERVICE_NAME = $(KUBERNETES_SERVICE_NAME)
dcos.setup-security.kubernetes: SECURITY_SERVICE_ACCOUNT_NAME = $(KUBERNETES_SERVICE_ACCOUNT_NAME)
dcos.setup-security.%:
ifeq ($(OPEN), yes)
	@echo "Skipping security setup since OPEN=yes..."
else ifeq ($(SECURITY), strict)
	@$(CURDIR)/hack/mke/setup_security_$*.sh $(SECURITY_SERVICE_NAME) $(SECURITY_SERVICE_ACCOUNT_NAME) --strict $(if $(FORCE_SETUP_SECURITY),--force)
else
	@$(CURDIR)/hack/mke/setup_security_$*.sh $(SECURITY_SERVICE_NAME) $(SECURITY_SERVICE_ACCOUNT_NAME) $(if $(__VERSION__),--force)
endif

install: kubernetes.package.install package.install edgelb.package.install

.PHONY: package.install
package.install: dcos.setup-security.kubernetes-cluster
ifeq ($(SECURITY), strict)
	@$(call retry,\
		dcos package install --yes $(KUBERNETES_CLUSTER_PACKAGE_NAME) --options=$(CURDIR)/hack/mke/kubernetes-cluster-strict-package-options.json,\
		8,\
		15)
else
	@$(call retry,\
		dcos package install --yes $(KUBERNETES_CLUSTER_PACKAGE_NAME),\
		8,\
		15)
endif
	$(MAKE) package.kubernetes-cluster.await-healthy

# NOTE: Kubernetes cluster package normally takes up 10. 10 minutes to be running and healthy.
package.kubernetes-cluster.await-healthy:
	@$(call retry,\
		dcos kubernetes cluster debug plan status deploy --cluster-name kubernetes-cluster --json 2> /dev/null | jq -e '.status == "COMPLETE"' &> /dev/null,\
		40,\
		15,\
		Kubernetes cluster installation isn't ready yet. Retrying in 15 seconds...)
	@echo "Kubernetes cluster package is healthy!"

# haproxy deploys haproxy with a specific config to the public agents in the
# cluster. it then makes sure the cli is installed and uses it to configure
# KUBECONFIG to access the kubernetes api.
.PHONY: haproxy
# Set SSH_PRIVATE_KEY_FILE to empty to stop terraform image starting SSH agent.
haproxy: SSH_PRIVATE_KEY_FILE=
haproxy:
# HAPROXY_PORT used when exposing the API server
# If the app already exists try to reuse the port to not break existing kubeconfigs
# Use the same range as Marathon would use
	$(eval HAPROXY_PORT ?= $(shell (dcos marathon app show /$(KUBERNETES_CLUSTER_SERVICE_NAME_DOMAIN)-kubernetes-haproxy | jq .portDefinitions[0].port) || echo 6443))
	$(eval PUBLIC_AGENTS_LOADBALANCER ?= $(shell echo "$$($(call run_terraform,output -state $(TERRAFORM_STATE_FILE) public-agents-loadbalancer))"))
	$(CURDIR)/hack/mke/haproxy/setup.sh \
		"$(PUBLIC_AGENTS_LOADBALANCER)" \
		"$(shell echo "$$($(call run_terraform,output -state $(TERRAFORM_STATE_FILE) public_agents_count))")" \
		$(KUBERNETES_CLUSTER_SERVICE_NAME_DOMAIN) \
		$(HAPROXY_PORT)
	@dcos package install --cli --yes $(KUBERNETES_PACKAGE_NAME)
	@dcos $(KUBERNETES_PACKAGE_NAME) cluster kubeconfig --cluster-name=$(SERVICE_NAME) --activate-context --apiserver-url="https://$(PUBLIC_AGENTS_LOADBALANCER):$(HAPROXY_PORT)" --context-name=$(SERVICE_NAME) --force-overwrite --insecure-skip-tls-verify
