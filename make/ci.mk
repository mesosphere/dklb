# DOCKER_CI_TAG holds the tag for the current Docker CI image calculated from the sha1 sum of
# Dockerfile.ci
DOCKER_CI_TAG := $(shell $(SHA1) Dockerfile.ci | awk '{ print $$1 }')

.PHONY: docker.ci.build
docker.ci.build: dockerauth
	@test "$(shell docker images -q mesosphere/dklb-ci:$(DOCKER_CI_TAG) 2> /dev/null)" != "" || \
		docker pull mesosphere/dklb-ci:$(DOCKER_CI_TAG) || \
		docker build \
		-f $(ROOT_DIR)/Dockerfile.ci \
		--build-arg VERSION=$(DOCKER_CI_TAG) \
		--tag "mesosphere/dklb-ci:$(DOCKER_CI_TAG)" $(ROOT_DIR)

.PHONY: docker.ci.push
docker.ci.push: docker.ci.build
	@docker push mesosphere/dklb-ci:$(DOCKER_CI_TAG)

.PHONY: docker.ci.run
docker.ci.run: RUN_WHAT ?=
docker.ci.run: docker.ci.build
	@docker run --rm -i$(if $(RUN_WHAT),,t) \
		-v $(ROOT_DIR):$(ROOT_DIR) \
		-w $(ROOT_DIR) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		$(if $(GITHUB_TOKEN),-e GITHUB_TOKEN=$(GITHUB_TOKEN)) \
		$(if $(DOCKER_USERNAME),-e DOCKER_USERNAME=$(DOCKER_USERNAME)) \
		$(if $(DOCKER_PASSWORD),-e DOCKER_PASSWORD=$(DOCKER_PASSWORD)) \
		$(if $(DCOS_KUBERNETES_CLUSTER_REGION),-e DCOS_KUBERNETES_CLUSTER_REGION=$(DCOS_KUBERNETES_CLUSTER_REGION)) \
		$(if $(AWS_REGION),-e AWS_REGION=$(AWS_REGION)) \
		-e SECURITY=$(SECURITY) \
		"mesosphere/dklb-ci:$(DOCKER_CI_TAG)" \
		$(RUN_WHAT)

.PHONY: dockerauth
dockerauth:
ifdef DOCKER_USERNAME
ifdef DOCKER_PASSWORD
	docker login -u $(DOCKER_USERNAME) -p $(DOCKER_PASSWORD)
endif
endif

.PHONY: ci.pre-commit
ci.pre-commit: gitauth
	@cd $$(mktemp -d) && go mod init tmp && go get mvdan.cc/sh/cmd/shfmt
	@go mod download
	@SKIP=no-commit-to-branch pre-commit run --all-files
