#!/bin/bash

set -e
set -x

# PUBLIC_AGENT_IP is the public ip of a public agent where haproxy will be
# deployed. we take the first element on the array for simplicity. a
# "$PUBLIC_AGENT_IP $SERVICE_NAME" entry will be added to /etc/hosts so that sni can
# be used.
PUBLIC_AGENT_IP=$1
# PUBLIC_AGENT_COUNT is the number of public agents in the dc/os cluster
PUBLIC_AGENT_COUNT=$2
# formatted a possible foldered service name to pass to HAProxy conf and /etc/hosts file
SERVICE_NAME=$3
# port to use in HAProxy marathon app
PORT=$4
# DIR is the path to the current directory
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# MARATHON_APP_ID must match the value in marathon.json
MARATHON_APP_ID="/${SERVICE_NAME}-kubernetes-haproxy"
# docker doesn't allow us to change /etc/hosts easily, so we must create a copy,
# edit it and copy (but not move) it back
cp /etc/hosts /tmp/hosts
# remove any matching entries (if any)
sed -i "/${SERVICE_NAME}/d" /tmp/hosts
# use a different marathon file
cp ${DIR}/marathon.json ${DIR}/generated-marathon.json
# replace the any maching entries with
sed -i "s|__SERVICE_NAME__|${SERVICE_NAME}|" ${DIR}/generated-marathon.json
sed -i "s|__PORT__|${PORT}|" ${DIR}/generated-marathon.json
# try to remove an existing marathon app with the same name
dcos marathon app remove ${MARATHON_APP_ID} || true
# deploy the marathon app
dcos marathon app add "${DIR}/generated-marathon.json"
# scale the app so we can be sure there is an instance at PUBLIC_AGENT_IP
dcos marathon app update ${MARATHON_APP_ID} instances=${PUBLIC_AGENT_COUNT}
# clean generated marathon
rm ${DIR}/generated-marathon.json
