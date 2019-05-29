#!/bin/bash
set -ex

# Configure the service account and permissions for kubernetes cluster
configure_kubernetes_cluster() {
  SERVICE_NAME=$1
  SERVICE_ACCOUNT_NAME=$2

  create_service_account $SERVICE_NAME $SERVICE_ACCOUNT_NAME $SERVICE_NAME/sa
  grant_kubernetes_cluster_permissions $SERVICE_NAME $SERVICE_ACCOUNT_NAME
}

# Grant permissions for the framework installation
grant_kubernetes_cluster_permissions() {
  SERVICE_NAME=$1
  SERVICE_ACCOUNT_NAME=$2

  # NOTE: Root user is required to run the kube-nodes
  USER="root"
  # Kubernetes role should be similar to the SERVICE_NAME by replacing the '/' with '__'..
  # The role cannot contain '/' in its name.
  KUBERNETES_ROLE=$(echo $SERVICE_NAME-role | sed 's/\//__/g')

  echo "Setting up permissions for $SERVICE_ACCOUNT_NAME with user $USER, service name $SERVICE_NAME and role $KUBERNETES_ROLE"

  # Allow the kubernetes framework to use the $KUBERNETES_ROLE
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:framework:role:$KUBERNETES_ROLE create
  # Grant the kubernetes tasks to run as `root` or the specified $USER
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:task:user:$USER create
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:agent:task:user:$USER create
  # Allow mesos reservation to be done with a role $KUBERNETES_ROLE and an account $SERVICE_ACCOUNT_NAME
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:reservation:role:$KUBERNETES_ROLE create
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:reservation:principal:$SERVICE_ACCOUNT_NAME delete
  # Grant the allocation of volumes for a role $KUBERNETES_ROLE and an account $SERVICE_ACCOUNT_NAME
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:volume:role:$KUBERNETES_ROLE create
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:volume:principal:$SERVICE_ACCOUNT_NAME delete

  # Grant the possibility to manage and list the secrets within a group with a $SERVICE_NAME
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:secrets:default:/$SERVICE_NAME/* full
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:secrets:list:default:/$SERVICE_NAME read

  # Grants a user access to the read-only endpoints of the DC/OS Certificate Authority API
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:adminrouter:ops:ca:rw full
  # Grants a user access to all endpoints of the DC/OS Certificate Authority API
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:adminrouter:ops:ca:ro full

  # Grant the permissions to run certain actions under the role slave_public/$KUBERNETES_ROLE
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:framework:role:slave_public/$KUBERNETES_ROLE create
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:framework:role:slave_public/$KUBERNETES_ROLE read
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:framework:role:slave_public read
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:agent:framework:role:slave_public read
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:reservation:role:slave_public/$KUBERNETES_ROLE create
  dcos security org users grant $SERVICE_ACCOUNT_NAME dcos:mesos:master:volume:role:slave_public/$KUBERNETES_ROLE create
}

KUBERNETES_CLUSTER_SERVICE_NAME="${1:-kubernetes-cluster}"
KUBERNETES_CLUSTER_SERVICE_ACCOUNT_NAME="${2:-kubernetes-cluster}"

DIR="${BASH_SOURCE%/*}"
if [[ ! -d "$DIR" ]]; then DIR="$PWD"; fi
. "$DIR/setup_security.sh"

configure_kubernetes_cluster $KUBERNETES_CLUSTER_SERVICE_NAME $KUBERNETES_CLUSTER_SERVICE_ACCOUNT_NAME

echo Done
