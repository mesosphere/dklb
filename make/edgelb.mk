# EDGELB_PACKAGE_NAME sets a default package name for edgelb
EDGELB_PACKAGE_NAME := edgelb

# EDGELB_SERVICE_ACCOUNT_NAME sets a default service account name for edgelb
EDGELB_SERVICE_ACCOUNT_NAME := edgelb-principal

# EDGELB_PACKAGE_VERSION sets the required package version
EDGELB_PACKAGE_VERSION := v1.3.1-269-g316af7d

.PHONY: edgelb.package.install
edgelb.package.install: dcos.setup-security.edgelb
	@dcos package repo remove edgelb-pool-aws || true
	@dcos package repo remove edgelb-aws || true
	@dcos package repo add --index=0 edgelb-aws https://edge-lb-infinity-artifacts.s3.amazonaws.com/saved/${EDGELB_PACKAGE_VERSION}/edgelb/stub-universe-edgelb.json
	@dcos package repo add --index=0 edgelb-pool-aws https://edge-lb-infinity-artifacts.s3.amazonaws.com/saved/${EDGELB_PACKAGE_VERSION}/edgelb-pool/stub-universe-edgelb-pool.json
ifeq ($(SECURITY), strict)
	@dcos package install --yes $(EDGELB_PACKAGE_NAME) --options=$(CURDIR)/hack/edgelb/package-options.json
else
	@dcos package install --yes $(EDGELB_PACKAGE_NAME)
endif
	$(MAKE) package.edgelb.await-healthy

dcos.setup-security.edgelb: SECURITY_SERVICE_NAME = ""
dcos.setup-security.edgelb: SECURITY_SERVICE_ACCOUNT_NAME = $(EDGELB_SERVICE_ACCOUNT_NAME)

# NOTE: Edgelb cluster package normally takes up 5 minutes to be running and healthy.
package.edgelb.await-healthy:
	for i in {1..20}; do dcos edgelb ping &> /dev/null && break || (echo "Edgelb installation isn't ready yet. Retrying in 15 seconds..." && sleep 15) ; done
	@echo "Edgelb package is healthy!"
