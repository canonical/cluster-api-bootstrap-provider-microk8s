#!/bin/bash -xe

# Usage:
#   $0 $http_proxy $https_proxy $no_proxy
#
# Assumptions:
#   - microk8s is installed

CONTAINERD_ENV="/var/snap/microk8s/current/args/containerd-env"

echo "# Configuration from ClusterAPI" >> "${CONTAINERD_ENV}"
need_restart=false

if [[ "${1}" != "" ]]; then
  echo "http_proxy=${1}" >> "${CONTAINERD_ENV}"
  echo "HTTP_PROXY=${1}" >> "${CONTAINERD_ENV}"
  need_restart=true
fi

if [[ "${2}" != "" ]]; then
  echo "https_proxy=${2}" >> "${CONTAINERD_ENV}"
  echo "HTTPS_PROXY=${2}" >> "${CONTAINERD_ENV}"
  need_restart=true
fi

if [[ "${3}" != "" ]]; then
  echo "no_proxy=${3}" >> "${CONTAINERD_ENV}"
  echo "NO_PROXY=${3}" >> "${CONTAINERD_ENV}"
  need_restart=true
fi

if [[ "$need_restart" = "true" ]]; then
  snap restart microk8s.daemon-containerd
fi
