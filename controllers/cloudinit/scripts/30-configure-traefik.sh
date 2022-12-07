#!/bin/bash -xe

# Usage:
#   $0 $endpoint $port
#
# Assumptions:
#   - microk8s is installed
#   - microk8s node has joined a cluster as a worker
#
# Notes:
#   - only required for microk8s <= 1.24

PROVIDER_YAML="/var/snap/microk8s/current/args/traefik/provider.yaml"

# cleanup any addresses from the provider.yaml file
sed '/address:/d' -i "${PROVIDER_YAML}"

# add the control plane to the list of addresses
# currently is using a hack since the list of endpoints is at the end of the file
echo "        - address: '${1}:${2}'" >> "${PROVIDER_YAML}"

# no restart is required, the file change is picked up automatically
