#!/bin/bash -xe

# Usage:
#   $0 $addon1 $addon2 [...]
#
# Assumptions:
#   - microk8s is installed
#   - microk8s apiserver is up and running

# enable community addons, this is for free and avoids confusion if addons are failing to install
microk8s enable community || true

while [[ "$@" != "" ]]; do
  microk8s enable "$1"
  microk8s status --wait-ready
  shift
done
