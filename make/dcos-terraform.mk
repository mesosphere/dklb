
# ----
# dcos-terraform specific variables
# these variables are only used and required when bootstrapping dc/os clusters
# using dcos-terraform
# ----

# DCOS_TERRAFORM_PLATFORM is the platform to run DC/OS Terraform against
DCOS_TERRAFORM_PLATFORM ?= aws

# TERRAFORM_DCOS_VERSION_gcp is the version of DC/OS Terraform GCP to run
TERRAFORM_DCOS_VERSION_gcp := 0.2.0

# TERRAFORM_DCOS_VERSION_aws is the version of DC/OS Terraform AWS to run
TERRAFORM_DCOS_VERSION_aws := 0.2.2

# TERRAFORM_VARS_FILE points to the dcos-terraform variables file that we'll use
# to bootstrap a cluster
TERRAFORM_VARS_FILE := $(CURDIR)/tools/dcos-terraform/terraform.tfvars

# TERRAFORM_VARS_FILE points to the dcos-terraform variables file that we'll use
TERRAFORM_PLAN_FILE := $(CURDIR)/tools/dcos-terraform/plan.out

# TERRAFORM_STATE_FILE points to the Terraform state file
TERRAFORM_STATE_FILE := $(CURDIR)/tools/dcos-terraform/terraform.tfstate

# SSH_PRIVATE_KEY_FILE points to the file that will contain the ssh key that can be used to
# ssh into agents in the dcos cluster
SSH_PRIVATE_KEY_FILE ?= $(CURDIR)/tools/dcos-terraform/.id_key

# DCOS_USERNAME contains the username to be used for authentication in dcos ee
# clusters
DCOS_USERNAME ?= admin

# DCOS_PASSWORD contains the password to be used for authentication in dcos ee
# clusters
DCOS_PASSWORD ?= deleteme

# DCOS_SUPERUSER_PASSWORD_HASH files with the generated dcos_superuser_password_hash
DCOS_SUPERUSER_PASSWORD_HASH_FILE := .dcos_superuser_password_hash

# DCOS_VERSION contains the version of DC/OS to install
DCOS_VERSION ?= 1.13.1

# DCOS_EE_LICENSE_PATH contains the path to the dcos ee license file.
DCOS_EE_LICENSE_PATH ?= $(CURDIR)dcos-ee-license.txt

# GOOGLE_APPLICATION_CREDENTIALS points at the file containing the gcp service
# account credentials to be used to launch clusters in gcp.
GOOGLE_APPLICATION_CREDENTIALS ?=

# GOOGLE_PROJECT is the GCP project to create resources in.
GOOGLE_PROJECT ?= massive-bliss-781

# DCOS_INSTANCE_OS is the instance OS for DC/OS
DCOS_INSTANCE_OS ?= centos_7.5

# DEFAULT_DCOS_MACHINE_TYPE_gcp is the default machine type on GCP.
DEFAULT_DCOS_MACHINE_TYPE_gcp := n1-standard-8

# DEFAULT_DCOS_BOOTSTRAP_MACHINE_TYPE_gcp is the default bootstrap machine type on GCP.
DEFAULT_DCOS_BOOTSTRAP_MACHINE_TYPE_gcp := n1-standard-4

# DEFAULT_DCOS_MACHINE_TYPE_aws is the default machine type on AWS.
DEFAULT_DCOS_MACHINE_TYPE_aws := m5.2xlarge

# DEFAULT_DCOS_BOOTSTRAP_MACHINE_TYPE_aws is the default bootstrap machine type on AWS.
DEFAULT_DCOS_BOOTSTRAP_MACHINE_TYPE_aws := m5.large

# DCOS_MACHINE_TYPE is the machine type to use for masters and nodes.
DCOS_MACHINE_TYPE ?= $(DEFAULT_DCOS_MACHINE_TYPE_$(DCOS_TERRAFORM_PLATFORM))

# DCOS_BOOTSTRAP_MACHINE_TYPE is the machine type to use for bootstrap machine.
DCOS_BOOTSTRAP_MACHINE_TYPE ?= $(DEFAULT_DCOS_BOOTSTRAP_MACHINE_TYPE_$(DCOS_TERRAFORM_PLATFORM))

# DEFAULT_DCOS_KUBERNETES_CLUSTER_REGION_gcp is the default GCP region to use.
DEFAULT_DCOS_KUBERNETES_CLUSTER_REGION_gcp := us-west1

# DEFAULT_DCOS_KUBERNETES_CLUSTER_REGION_aws is the default AWS region to use.
DEFAULT_DCOS_KUBERNETES_CLUSTER_REGION_aws := us-west-2

# DCOS_KUBERNETES_CLUSTER_REGION is the region to use.
DCOS_KUBERNETES_CLUSTER_REGION ?= $(DEFAULT_DCOS_KUBERNETES_CLUSTER_REGION_$(DCOS_TERRAFORM_PLATFORM))

# NUM_PRIVATE_AGENTS is the number of dcos private agents we want to have in the
# dcos cluster launched by dcos-terraform
NUM_PRIVATE_AGENTS ?= 3

# NUM_PRIVATE_NODES is the number of private kubernetes nodes we want to have in
# the kubernetes cluster. it defaults to the number of dcos private agents.
NUM_PRIVATE_NODES ?= $(NUM_PRIVATE_AGENTS)

# NUM_PUBLIC_AGENTS is the number of dcos public agents we want to have in the
# dcos cluster launched by dcos-terraform
NUM_PUBLIC_AGENTS ?= 1

# NUM_PUBLIC_NODES is the number of public kubernetes nodes we want to have in
# the kubernetes cluster. it defaults to the number of dcos public agents.
NUM_PUBLIC_NODES ?= $(NUM_PUBLIC_AGENTS)

# NUM_MASTERS is the number of dcos masters we want to have in the dcos cluster
# launched by dcos-terraform
NUM_MASTERS ?= 1

# SECURITY is the security mode to use for dcos ee clusters launched by
# dcos-terraform. valid values are permissive and strict
SECURITY ?= strict

# DCOS_TERRAFORM_ENVIRONMENT_VARS_gcp is the necessary environment variables for GCP.
DCOS_TERRAFORM_ENVIRONMENT_VARS_gcp := -e GOOGLE_APPLICATION_CREDENTIALS=$(GOOGLE_APPLICATION_CREDENTIALS) \
	-e GOOGLE_PROJECT=$(GOOGLE_PROJECT) \
	-e GOOGLE_REGION=$(DCOS_KUBERNETES_CLUSTER_REGION)

# AWS_SHARED_CREDENTIALS_FILE is the AWS shared credentials file.
AWS_SHARED_CREDENTIALS_FILE ?= $(HOME)/.aws/credentials

# AWS_PROFILE is the profile to use if no default.
AWS_PROFILE ?=

# DCOS_TERRAFORM_ENVIRONMENT_VARS_aws is the necessary environment variables for AWS.
DCOS_TERRAFORM_ENVIRONMENT_VARS_aws := -e AWS_REGION=$(DCOS_KUBERNETES_CLUSTER_REGION) \
	-e AWS_PROFILE=$(AWS_PROFILE) \
	-e AWS_SHARED_CREDENTIALS_FILE=$(AWS_SHARED_CREDENTIALS_FILE)

# CUSTOM_DCOS_DOWNLOAD_PATH specifies a custom download path for DC/OS installer, used to install
# development versions or versions not yet supported by terraform modules.
CUSTOM_DCOS_DOWNLOAD_PATH ?=

# DCOS_CONFIG holds extra DC/OS configuration.
DCOS_CONFIG ?=

# DKLB_IAM_POLICY_NAME is the DKLB IAM policy name to use.
DKLB_IAM_POLICY_NAME ?=

# ----
# dcos-terraform specific targets
# ----

# configures the dcos cli to point at the dc/os cluster
.PHONY: setup-cli
setup-cli: SSH_PRIVATE_KEY_FILE=
setup-cli: dockerauth dcos-cli
	$(eval MASTER_IP := $(shell echo "$$($(call run_terraform,output -state $(TERRAFORM_STATE_FILE) cluster-address))"))
ifeq ($(OPEN),yes)
	python3 -m venv env && \
	. env/bin/activate && \
	pip3 install pip==$(PYTHON_PIP_VERSION) && \
	pip3 install -r tools/dcos_open_login/requirements.txt && \
	CLUSTER_URL=https://$(MASTER_IP) DCOS_ENTERPRISE=false python3 tools/dcos_open_login/dcos_login.py
else
	$(call retry,dcos cluster setup --insecure --username=$(DCOS_USERNAME) --password=$(DCOS_PASSWORD) https://$(MASTER_IP),12,5)
# Wait until the cluster is really available. Strict mode requires some more time due to
# asynchronous configuration happening after cluster is created.
	$(call retry,[ "$$(curl -fsSLk $$(dcos config show core.dcos_url)/system/health/v1/units -H "Authorization: Bearer $$(dcos config show core.dcos_acs_token)" | jq '[.units[] | select(.health != 0)] | length')" == 0 ],60,5)
endif

.PHONY: dcos-cli
dcos-cli:
	@command -v dcos &> /dev/null || \
		(curl -fsSLo /usr/local/bin/dcos https://downloads.dcos.io/binaries/cli/linux/x86-64/dcos-1.13/dcos && \
			chmod +x /usr/local/bin/dcos)

# to understand whether we're running inside or outside of teamcity, we check
# whether THIS_BUILD_NUM is set. when performing a build, teamcity automatically
# sets it to the number of the current build, so it is a good way to check for
# this without having to manually add an envvar to every job's configuration.
ifeq ($(THIS_BUILD_NUM),)
# we're most probably running outside teamcity, so default the owner to
# "mesosphere".
DCOS_CLUSTER_EXPIRATION ?= 7d
DCOS_CLUSTER_OWNER ?= $(HOST_USER)
DCOS_CLUSTER_PREFIX ?= dklb
else
# we're most probably running inside teamcity, so default the owner to
# "teamcity".
DCOS_CLUSTER_EXPIRATION ?= 1d
DCOS_CLUSTER_OWNER ?= teamcity
DCOS_CLUSTER_PREFIX ?= dklb-ci
endif

ifeq ($(OPEN),yes)
DCOS_VARIANT=open
else
DCOS_VARIANT=ee
endif

define dklb_instance_profile
{
	"Version": "2012-10-17",
	"Statement": [{
		"Action": [
			"elasticloadbalancing:DescribeLoadBalancers",
			"elasticloadbalancing:CreateLoadBalancer",
			"elasticloadbalancing:DeleteLoadBalancer",
			"elasticloadbalancing:DescribeListeners",
			"elasticloadbalancing:CreateListener",
			"elasticloadbalancing:DeleteListener",
			"elasticloadbalancing:ModifyListener",
			"elasticloadbalancing:CreateTargetGroup",
			"elasticloadbalancing:DeleteTargetGroup",
			"elasticloadbalancing:DescribeTargetGroups",
			"elasticloadbalancing:ModifyTargetGroup",
			"elasticloadbalancing:RegisterTargets",
			"elasticloadbalancing:DeregisterTargets",
			"elasticloadbalancing:DescribeTargetHealth",
			"elasticloadbalancing:DescribeLoadBalancerAttributes",
			"elasticloadbalancing:ModifyLoadBalancerAttributes",
			"elasticloadbalancing:DescribeTags",
			"elasticloadbalancing:AddTags",
			"elasticloadbalancing:RemoveTags"
		],
		"Resource": "*",
		"Effect": "Allow"
	}]
}
endef

# launches a dc/os cluster
TMPFILE := $(shell mktemp)
export dklb_instance_profile
CLIENT_PUBLIC_IP ?= $(shell curl -fsSL https://checkip.amazonaws.com)
export AWS_DEFAULT_REGION ?= $(DCOS_KUBERNETES_CLUSTER_REGION)
launch-dcos: $(TERRAFORM_VARS_FILE)
	$(call run_terraform,plan -state $(TERRAFORM_STATE_FILE) -out $(TERRAFORM_PLAN_FILE))
	$(call run_terraform,apply -state-out $(TERRAFORM_STATE_FILE) -auto-approve $(TERRAFORM_PLAN_FILE))
	@echo "$${dklb_instance_profile}" > $(TMPFILE)
	@export DKLB_INSTANCE_POLICY_NAME="dcos-$(DCOS_CLUSTER_PREFIX)-$(shell cat .random_cluster_name_suffix)-dklb_instance_policy" && \
		export DKLB_INSTANCE_POLICY_ARN=$$(aws iam list-policies | jq -r ".Policies[] | select(.PolicyName == \"$${DKLB_INSTANCE_POLICY_NAME}\") | .Arn") && \
		( \
			[ -z "$${DKLB_INSTANCE_POLICY_ARN}" ] || \
			( \
				aws iam detach-role-policy --role-name dcos-$(DCOS_CLUSTER_PREFIX)-$(shell cat .random_cluster_name_suffix)-instance_role \
					--policy-arn $${DKLB_INSTANCE_POLICY_ARN} && \
				aws iam delete-policy --policy-arn $${DKLB_INSTANCE_POLICY_ARN} \
			) \
		) && \
		aws iam attach-role-policy --role-name dcos-$(DCOS_CLUSTER_PREFIX)-$(shell cat .random_cluster_name_suffix)-instance_role \
			--policy-arn $$(aws iam create-policy --policy-name $${DKLB_INSTANCE_POLICY_NAME} --policy-document file://$(TMPFILE) | jq -r '.Policy.Arn')
	@$(RM) $(TMPFILE)
	@aws ec2 revoke-security-group-ingress \
		--group-id $$(aws ec2 describe-security-groups | jq -r '.SecurityGroups[] | select(.GroupName == "dcos-$(DCOS_CLUSTER_PREFIX)-$(shell cat .random_cluster_name_suffix)-internal-firewall") | .GroupId') \
		--protocol tcp --port 1025-65535 --cidr $(CLIENT_PUBLIC_IP)/32 &> /dev/null \
		|| true
	@aws ec2 authorize-security-group-ingress \
		--group-id $$(aws ec2 describe-security-groups | jq -r '.SecurityGroups[] | select(.GroupName == "dcos-$(DCOS_CLUSTER_PREFIX)-$(shell cat .random_cluster_name_suffix)-internal-firewall") | .GroupId') \
		--protocol tcp --port 1025-65535 --cidr $(CLIENT_PUBLIC_IP)/32

.random_cluster_name_suffix:
	@openssl rand -base64 100 | tr -dc 'a-zA-Z0-9' | fold -w 6 | head -n 1 > $@

# generates the terraform.tfvars file that is later used by the launch-dcos target.
$(TERRAFORM_VARS_FILE): $(DCOS_SUPERUSER_PASSWORD_HASH_FILE) .random_cluster_name_suffix
	@DCOS_VARIANT=$(DCOS_VARIANT) \
	DCOS_VERSION=$(DCOS_VERSION) \
	DCOS_SECURITY=$(SECURITY) \
	DCOS_INSTANCE_OS=$(DCOS_INSTANCE_OS) \
	DCOS_VERSION=$(DCOS_VERSION) \
	DCOS_CLUSTER_EXPIRATION=$(DCOS_CLUSTER_EXPIRATION) \
	DCOS_CLUSTER_OWNER=$(DCOS_CLUSTER_OWNER) \
	DCOS_CLUSTER_NAME=$(DCOS_CLUSTER_PREFIX)-$(shell cat .random_cluster_name_suffix) \
	DCOS_EE_LICENSE_PATH=$(DCOS_EE_LICENSE_PATH) \
	DCOS_MACHINE_TYPE=$(DCOS_MACHINE_TYPE) \
	DCOS_BOOTSTRAP_MACHINE_TYPE=$(DCOS_BOOTSTRAP_MACHINE_TYPE) \
	NUM_MASTERS=$(NUM_MASTERS) \
	NUM_PRIVATE_AGENTS=$(NUM_PRIVATE_AGENTS) \
	NUM_PUBLIC_AGENTS=$(NUM_PUBLIC_AGENTS) \
	DCOS_SUPERUSER_USERNAME=$(DCOS_USERNAME) \
	DCOS_SUPERUSER_PASSWORD_HASH='$(shell cat $(DCOS_SUPERUSER_PASSWORD_HASH_FILE))' \
	GOOGLE_APPLICATION_CREDENTIALS=$(GOOGLE_APPLICATION_CREDENTIALS) \
	AWS_SHARED_CREDENTIALS_FILE=$(AWS_SHARED_CREDENTIALS_FILE) \
	SSH_PRIVATE_KEY_FILE=$(SSH_PRIVATE_KEY_FILE) \
	SSH_PUBLIC_KEY_FILE=$(SSH_PUBLIC_KEY_FILE) \
	DCOS_TERRAFORM_PLATFORM=$(DCOS_TERRAFORM_PLATFORM) \
	CUSTOM_DCOS_DOWNLOAD_PATH=$(CUSTOM_DCOS_DOWNLOAD_PATH) \
	DCOS_CONFIG="$(DCOS_CONFIG)" \
	DKLB_IAM_POLICY_NAME="$(DKLB_IAM_POLICY_NAME)" \
	$(CURDIR)/tools/dcos-terraform/init.sh

$(DCOS_SUPERUSER_PASSWORD_HASH_FILE):
	@docker run bernadinm/sha512 '$(DCOS_PASSWORD)' > $(DCOS_SUPERUSER_PASSWORD_HASH_FILE)

# destroys the dc/os cluster
export AWS_DEFAULT_REGION ?= $(DCOS_KUBERNETES_CLUSTER_REGION)
destroy-dcos: $(TERRAFORM_VARS_FILE)
	@export DKLB_INSTANCE_POLICY_NAME="dcos-$(DCOS_CLUSTER_PREFIX)-$(shell cat .random_cluster_name_suffix)-dklb_instance_policy" && \
		export DKLB_INSTANCE_POLICY_ARN=$$(aws iam list-policies | jq -r ".Policies[] | select(.PolicyName == \"$${DKLB_INSTANCE_POLICY_NAME}\") | .Arn") && \
		( \
			[ -z "$${DKLB_INSTANCE_POLICY_ARN}" ] || \
			( \
				aws iam detach-role-policy --role-name dcos-$(DCOS_CLUSTER_PREFIX)-$(shell cat .random_cluster_name_suffix)-instance_role \
					--policy-arn $${DKLB_INSTANCE_POLICY_ARN} && \
				aws iam delete-policy --policy-arn $${DKLB_INSTANCE_POLICY_ARN} \
			) \
		)
	$(call run_terraform,destroy -state $(TERRAFORM_STATE_FILE) -auto-approve)
	@git clean -fdx $(CURDIR)/tools/dcos-terraform
	@$(RM) .random_cluster_name_suffix

# destroys the dc/os cluster and wipes build artifacts
clean-all: destroy-dcos

# launches a dc/os cluster
setup: launch-dcos setup-cli install

# fetches a number of logs from a running dc/os cluster
get-logs: setup-cli
	tools/get_logs.sh tools/dcos-terraform/.id_key centos
	@mkdir -p logs
	@cp tools/dcos-terraform/.id_key logs/

# Runs terraform in a docker container
define run_terraform
docker run --rm -i \
	-v $(TERRAFORM_VARS_FILE):/dcos-terraform/terraform.tfvars \
	-v $(CURDIR)/tools/dcos-terraform/extra-outputs.tf:/dcos-terraform/extra-outputs.tf \
	-v $(CURDIR):$(CURDIR) \
	$(if $(wildcard $(GOOGLE_APPLICATION_CREDENTIALS)),-v $(GOOGLE_APPLICATION_CREDENTIALS):$(GOOGLE_APPLICATION_CREDENTIALS)) \
	$(if $(wildcard $(AWS_SHARED_CREDENTIALS_FILE)),-v $(AWS_SHARED_CREDENTIALS_FILE):$(AWS_SHARED_CREDENTIALS_FILE) -e AWS_SHARED_CREDENTIALS_FILE=$(AWS_SHARED_CREDENTIALS_FILE)) \
	$(if $(AWS_PROFILE),-e AWS_PROFILE=$(AWS_PROFILE)) \
	$(if $(AWS_SECRET_ACCESS_KEY),-e AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY)) \
	$(if $(AWS_ACCESS_KEY_ID),-e AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID)) \
	$(DCOS_TERRAFORM_ENVIRONMENT_VARS_$(DCOS_TERRAFORM_PLATFORM)) \
	$(if $(DCOS_EE_LICENSE_PATH),-v $(DCOS_EE_LICENSE_PATH):$(DCOS_EE_LICENSE_PATH)) \
	$(if $(SSH_PUBLIC_KEY_FILE),-v $(SSH_PUBLIC_KEY_FILE):$(SSH_PUBLIC_KEY_FILE)) \
	$(if $(SSH_PRIVATE_KEY_FILE),-v $(SSH_PRIVATE_KEY_FILE):$(SSH_PRIVATE_KEY_FILE) -e SSH_PRIVATE_KEY_FILE=$(SSH_PRIVATE_KEY_FILE)) \
	-u $(shell id -u):$(shell id -g) \
	mesosphere/dcos-terraform-$(DCOS_TERRAFORM_PLATFORM):v$(TERRAFORM_DCOS_VERSION_$(DCOS_TERRAFORM_PLATFORM)) \
	$1
endef

# retry runs the specified command a number of times before exiting.
# $(1): command to run
# $(2): number of attempts
# $(3): sleep time between attempts
define retry
	ATTEMPTS=0; \
	until $(1); do \
		[ -n "$(4)" ] && echo "$(4)" || true; \
		if [ $$(( ATTEMPTS++ )) -eq $(2) ]; then \
			exit 1; \
		fi; \
		sleep $(3); \
	done
endef
