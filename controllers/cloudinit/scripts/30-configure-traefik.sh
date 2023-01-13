#!/bin/bash -xe

# Usage:
#   $0 $endpoint $port $stop_ep_refresh
#
# Assumptions:
#   - microk8s is installed
#   - microk8s node has joined a cluster as a worker
#
# Notes:
#   - stopping API servers endpoint refreshes should be done only on for 1.25+

PROVIDER_YAML="/var/snap/microk8s/current/args/traefik/provider.yaml"
APISERVER_PROXY_ARGS_FILE="/var/snap/microk8s/current/args/apiserver-proxy"

while ! [ -f "${PROVIDER_YAML}" ]; do
    echo "Waiting for ${PROVIDER_YAML}"
    sleep 5
done

if [ ${3} == "yes" ]; then
  sed '/refresh-interval/d' -i "${APISERVER_PROXY_ARGS_FILE}"
  echo "--refresh-interval 0s" >> "${APISERVER_PROXY_ARGS_FILE}"
  snap restart microk8s.daemon-apiserver-proxy
fi

# cleanup any addresses from the provider.yaml file
sed '/address:/d' -i "${PROVIDER_YAML}"

# add the control plane to the list of addresses
# currently is using a hack since the list of endpoints is at the end of the file
echo "        - address: '${1}:${2}'" >> "${PROVIDER_YAML}"
# no restart is required, the file change is picked up automatically
