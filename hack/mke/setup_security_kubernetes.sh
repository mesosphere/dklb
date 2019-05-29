#!/bin/bash
set -ex

# Configure the service account and permissions for kubernetes
configure_kubernetes() {
  SERVICE_NAME=$1
  SERVICE_ACCOUNT_NAME=$2

  create_service_account $SERVICE_NAME $SERVICE_ACCOUNT_NAME $SERVICE_NAME/sa
  grant_kubernetes_permissions $SERVICE_ACCOUNT_NAME
}

# Grant permissions for kubernetes
grant_kubernetes_permissions() {
  SERVICE_ACCOUNT_NAME=$1

  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:reservation:role:$SERVICE_ACCOUNT_NAME-role create
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:framework:role:$SERVICE_ACCOUNT_NAME-role create
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:reservation:principal:$SERVICE_ACCOUNT_NAME delete
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:task:user:nobody create
}

KUBERNETES_SERVICE_NAME="${1:-kubernetes}"
KUBERNETES_SERVICE_ACCOUNT_NAME="${2:-kubernetes}"

DIR="${BASH_SOURCE%/*}"
if [[ ! -d "$DIR" ]]; then DIR="$PWD"; fi
. "$DIR/setup_security.sh"

configure_kubernetes $KUBERNETES_SERVICE_NAME $KUBERNETES_SERVICE_ACCOUNT_NAME

echo Done
