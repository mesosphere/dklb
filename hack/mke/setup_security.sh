#!/bin/bash
set -ex

# Create the service account and the service account secret
create_service_account() {
  SERVICE_NAME=$1
  SERVICE_ACCOUNT_NAME=$2
  SECRET_NAME=$3

  if dcos security org service-accounts show "${SERVICE_ACCOUNT_NAME}" &>/dev/null; then
    if [ "$FORCE" != "y" ]; then
      echo "Service account ${SERVICE_ACCOUNT_NAME} already exists so not recreating"
      echo "If you wish to force recreation or resetting permissions please run:"
      echo "${0} --force"
      echo "Or if you're using this via make please run:"
      echo "make FORCE_SETUP_SECURITY=1 <GOAL>"
      exit 0
    fi
  fi

  echo Create keypair...
  if ! dcos security org service-accounts keypair private-key.pem public-key.pem; then
    echo "Failed to create keypair for testing service account" >&2
    exit 1
  fi

  echo Creating service account for account=$SERVICE_ACCOUNT_NAME secret=$SECRET_NAME mode=$MODE

  echo Create service account...
  dcos security org service-accounts delete "${SERVICE_ACCOUNT_NAME}" &>/dev/null || true
  if ! dcos security org service-accounts create -p public-key.pem -d "${SERVICE_ACCOUNT_NAME} service account" "${SERVICE_ACCOUNT_NAME}"; then
    echo "Failed to create service account '${SERVICE_ACCOUNT_NAME}'" >&2
    exit 1
  fi

  echo Create secret...
  dcos security secrets delete "${SECRET_NAME}" &>/dev/null || true
  if ! dcos security secrets create-sa-secret ${MODE} private-key.pem "${SERVICE_ACCOUNT_NAME}" "${SECRET_NAME}"; then
    echo "Failed to create secret '${SECRET_NAME}' for service account '${SERVICE_ACCOUNT_NAME}'" >&2
    exit 1
  fi

  echo Service $SERVICE_NAME with account created for account=$SERVICE_ACCOUNT_NAME secret=$SECRET_NAME
}

MODE=
FORCE=

# Otherwise, Python will complain.
export LC_ALL=en_US.UTF-8
export LANG=en_US.UTF-8

while [ ! $# -eq 0 ]; do
  case "$1" in
    --strict | -s)
      MODE="--strict"
      ;;
    --force | -f)
      FORCE="y"
      ;;
  esac
  shift
done

if ! dcos security --version; then
  echo Install cli necessary for security...
  if ! dcos package install dcos-enterprise-cli --yes; then
    echo "Failed to install dcos-enterprise cli extension" >&2
    exit 1
  fi
else
  echo dcos security subcommand is available so not installing
fi

echo Done
