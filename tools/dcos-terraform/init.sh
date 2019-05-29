#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'

# required environment variables:
# - DCOS_EE_LICENSE_PATH
#   - required if DCOS_VARIANT is ee
# - DCOS_SUPERUSER_PASSWORD_HASH (default: the hash of the default from DC/OS that we all know :))
# - DCOS_ADMIN_IPS
#   - Space-separate list of CIDR admin IPs
# - GOOGLE_APPLICATION_CREDENTIALS

# optional environment variables:
# - DCOS_TERRAFORM_PLATFORM (default: aws)
#   - valid values are gcp and aws
DCOS_TERRAFORM_PLATFORM=${DCOS_TERRAFORM_PLATFORM:-aws}
# - DCOS_VARIANT (default: ee)
#   - valid values are open and ee
DCOS_VARIANT=${DCOS_VARIANT:-ee}
# - DCOS_VERSION (default: 1.12.3)
DCOS_VERSION=${DCOS_VERSION:-1.12.3}
# - DCOS_SECURITY (default: strict)
#   - valid values are strict, permissive, disabled
DCOS_SECURITY=${DCOS_SECURITY:-strict}
# - DCOS_INSTANCE_OS (default: centos_7.5)
#   - valid values are coreos_1855.5.0, centos_7.5
DCOS_INSTANCE_OS=${DCOS_INSTANCE_OS:-centos_7.5}
# - DCOS_CLUSTER_NAME (default: dcos-kubernetes)
DCOS_CLUSTER_NAME=${DCOS_CLUSTER_NAME:-dcos-kubernetes}
# - DCOS_CLUSTER_NAME_RANDOM_STRING (default: true)
#   - valid values are true, false
DCOS_CLUSTER_NAME_RANDOM_STRING=${DCOS_CLUSTER_NAME_RANDOM_STRING:-true}
# - DCOS_MACHINE_TYPE is the instance type to be used (default: n1-standard-8)
DCOS_MACHINE_TYPE=${DCOS_MACHINE_TYPE:-n1-standard-8}
# - DCOS_BOOTSTRAP_MACHINE_TYPE (default: n1-standard-2)
DCOS_BOOTSTRAP_MACHINE_TYPE=${DCOS_BOOTSTRAP_MACHINE_TYPE:-n1-standard-2}
# - NUM_MASTERS (default: 1)
#   - can be any positive integer
NUM_MASTERS=${NUM_MASTERS:-1}
# - NUM_PRIVATE_AGENTS (default: 3)
#   - can be any positive integer
NUM_PRIVATE_AGENTS=${NUM_PRIVATE_AGENTS:-3}
# - NUM_PUBLIC_AGENTS (default: 0)
#   - can be any non-negative integer
NUM_PUBLIC_AGENTS=${NUM_PUBLIC_AGENTS:-0}
# - DCOS_PRIVATE_AGENTS_MACHINE_TYPE (default: same as DCOS_MACHINE_TYPE)
DCOS_PRIVATE_AGENTS_MACHINE_TYPE=${DCOS_PRIVATE_AGENTS_MACHINE_TYPE:-${DCOS_MACHINE_TYPE}}
# - DCOS_PUBLIC_AGENTS_MACHINE_TYPE (default: same as DCOS_MACHINE_TYPE)
DCOS_PUBLIC_AGENTS_MACHINE_TYPE=${DCOS_PUBLIC_AGENTS_MACHINE_TYPE:-${DCOS_MACHINE_TYPE}}
# - DCOS_MASTERS_MACHINE_TYPE (default: same as DCOS_MACHINE_TYPE)
DCOS_MASTERS_MACHINE_TYPE=${DCOS_MASTERS_MACHINE_TYPE:-${DCOS_MACHINE_TYPE}}
# - DCOS_SUPERUSER_USERNAME (default: admin)
DCOS_SUPERUSER_USERNAME=${DCOS_SUPERUSER_USERNAME:-admin}
# - REGION (default: us-west1)
REGION=${REGION:-us-west1}
# - SSH_PUBLIC_KEY_FILE (if unset, one will be generated)
SSH_PUBLIC_KEY_FILE=${SSH_PUBLIC_KEY_FILE:-}
# - CUSTOM_DCOS_DOWNLOAD_PATH
CUSTOM_DCOS_DOWNLOAD_PATH=${CUSTOM_DCOS_DOWNLOAD_PATH:-}
# - DCOS_CONFIG is the custom DC/OS , e.g. to enable seccomp
DCOS_CONFIG=${DCOS_CONFIG:-}
# - DCOS_CLUSTER_OWNER is the owner tag in AWS
DCOS_CLUSTER_OWNER=${DCOS_CLUSTER_OWNER:-kubernetes-team}
# - DCOS_CLUSTER_EXPIRATION is the expiration tag in AWS for cloud cleaner
DCOS_CLUSTER_EXPIRATION=${DCOS_CLUSTER_EXPIRATION:-1d}

function fail() {
  echo >&2 "FATAL: ${1}"
  exit 1
}
function warn() { echo >&2 "WARN: ${1}"; }

# SCRIPT_DIR stores the absolute path to this script's directory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# validate the value of the OPEN envvar
case "${DCOS_VARIANT}" in
  "ee")
    # check for the existence of the license file and warn if we can't find it
    if [[ ! -f "${DCOS_EE_LICENSE_PATH}" ]]; then
      fail "DCOS_EE_LICENSE_PATH does not exist"
    fi
    ;;
  "open") ;;

  *)
    fail "must set DCOS_VARIANT to one of [ee,open]"
    ;;
esac

# validate the value of the DCOS_SECURITY envvar
case "${DCOS_SECURITY}" in
  "strict" | "permissive" | "disabled")
    # nothing to do here
    ;;
  *)
    fail "must set DCOS_SECURITY to one of [strict,permissive,disabled]"
    ;;
esac

NUM_MASTERS=${NUM_MASTERS:-1}
# validate the value of the NUM_MASTERS envvar
if ((NUM_MASTERS < 1)); then
  fail "NUM_MASTERS must be a positive integer"
fi

NUM_PRIVATE_AGENTS=${NUM_PRIVATE_AGENTS:-3}
# validate the value of the NUM_PRIVATE_AGENTS envvar
if ((NUM_PRIVATE_AGENTS < 1)); then
  fail "NUM_PRIVATE_AGENTS must be a positive integer"
fi

NUM_PUBLIC_AGENTS=${NUM_PUBLIC_AGENTS:-1}
# validate the value of the NUM_PUBLIC_AGENTS envvar
if ((NUM_PUBLIC_AGENTS < 0)); then
  fail "NUM_PUBLIC_AGENTS must be a non-negative integer"
fi

if [ -z "${SSH_PUBLIC_KEY_FILE:-}" ]; then
  SSH_PRIVATE_KEY_FILE=${SCRIPT_DIR}/.id_key
  if [ ! -r ${SSH_PRIVATE_KEY_FILE} ]; then
    ssh-keygen -t rsa -f ${SSH_PRIVATE_KEY_FILE} -N ''
  fi

  SSH_PUBLIC_KEY_FILE=${SSH_PRIVATE_KEY_FILE}.pub
fi

echo ${DCOS_TERRAFORM_PLATFORM}

# create the 'terraform.tfvars' file with desired cluster configuration
case "${DCOS_TERRAFORM_PLATFORM}" in
  "gcp")
    cat <<EOF >"${SCRIPT_DIR}/terraform.tfvars"
cluster_name                    = "${DCOS_CLUSTER_NAME}"
cluster_name_random_string      = ${DCOS_CLUSTER_NAME_RANDOM_STRING}
dcos_variant                    = "${DCOS_VARIANT}"
dcos_license_key_file           = "${DCOS_EE_LICENSE_PATH}"
dcos_version                    = "${DCOS_VERSION}"
custom_dcos_download_path       = "${CUSTOM_DCOS_DOWNLOAD_PATH}"
dcos_security                   = "${DCOS_SECURITY}"
dcos_instance_os                = "${DCOS_INSTANCE_OS}"
bootstrap_machine_type          = "${DCOS_BOOTSTRAP_MACHINE_TYPE}"
masters_machine_type            = "${DCOS_MASTERS_MACHINE_TYPE}"
private_agents_machine_type     = "${DCOS_PRIVATE_AGENTS_MACHINE_TYPE}"
public_agents_machine_type      = "${DCOS_PUBLIC_AGENTS_MACHINE_TYPE}"
num_masters                     = "${NUM_MASTERS}"
num_public_agents               = "${NUM_PUBLIC_AGENTS}"
num_private_agents              = "${NUM_PRIVATE_AGENTS}"
dcos_superuser_username         = "${DCOS_SUPERUSER_USERNAME}"
dcos_superuser_password_hash    = "${DCOS_SUPERUSER_PASSWORD_HASH:-}"
ssh_public_key_file             = "${SSH_PUBLIC_KEY_FILE}"
admin_ips                       = "${DCOS_ADMIN_IPS:-}"
public_agents_additional_ports  = ["6443"]
dcos_config                     = $(echo -e ${DCOS_CONFIG})
EOF
    ;;
  "aws")
    cat <<EOF >"${SCRIPT_DIR}/terraform.tfvars"
cluster_name                    = "${DCOS_CLUSTER_NAME}"
cluster_name_random_string      = ${DCOS_CLUSTER_NAME_RANDOM_STRING}
dcos_variant                    = "${DCOS_VARIANT}"
dcos_license_key_file           = "${DCOS_EE_LICENSE_PATH}"
dcos_version                    = "${DCOS_VERSION}"
custom_dcos_download_path       = "${CUSTOM_DCOS_DOWNLOAD_PATH}"
dcos_security                   = "${DCOS_SECURITY}"
dcos_instance_os                = "${DCOS_INSTANCE_OS}"
bootstrap_instance_type         = "${DCOS_BOOTSTRAP_MACHINE_TYPE}"
masters_instance_type           = "${DCOS_MASTERS_MACHINE_TYPE}"
private_agents_instance_type    = "${DCOS_PRIVATE_AGENTS_MACHINE_TYPE}"
public_agents_instance_type     = "${DCOS_PUBLIC_AGENTS_MACHINE_TYPE}"
num_masters                     = "${NUM_MASTERS}"
num_public_agents               = "${NUM_PUBLIC_AGENTS}"
num_private_agents              = "${NUM_PRIVATE_AGENTS}"
dcos_superuser_username         = "${DCOS_SUPERUSER_USERNAME}"
dcos_superuser_password_hash    = "${DCOS_SUPERUSER_PASSWORD_HASH:-}"
ssh_public_key_file             = "${SSH_PUBLIC_KEY_FILE}"
admin_ips                       = "${DCOS_ADMIN_IPS:-}"
public_agents_additional_ports  = ["6443"]
dcos_config                     = $(echo -e ${DCOS_CONFIG:-\"\"})
tags                            = {
                                    owner      = "${DCOS_CLUSTER_OWNER}"
                                    expiration = "${DCOS_CLUSTER_EXPIRATION}"
                                  }
EOF
    ;;
  *)
    fail "must set DCOS_TERRAFORM_PLATFORM to one of [gcp,aws]"
    ;;
esac
