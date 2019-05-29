#!/bin/bash
set -ex

# Configure the service account and permissions for edgelb
configure_edgelb() {
  SERVICE_NAME=$1
  SERVICE_ACCOUNT_NAME=$2

  create_service_account $SERVICE_NAME $SERVICE_ACCOUNT_NAME edgelb-secret
  grant_edgelb_permissions $SERVICE_ACCOUNT_NAME
}

# Grant permissions for kubernetes
grant_edgelb_permissions() {
  SERVICE_ACCOUNT_NAME=$1

  dcos security org groups add_user superusers $SERVICE_ACCOUNT_NAME
}

EDGELB_SERVICE_NAME="${1:-edgelb}"
EDGELB_SERVICE_ACCOUNT_NAME="${2:-edgelb-principal}"

DIR="${BASH_SOURCE%/*}"
if [[ ! -d "$DIR" ]]; then DIR="$PWD"; fi
. "$DIR/setup_security.sh"

configure_edgelb $EDGELB_SERVICE_NAME $EDGELB_SERVICE_ACCOUNT_NAME

echo Done
