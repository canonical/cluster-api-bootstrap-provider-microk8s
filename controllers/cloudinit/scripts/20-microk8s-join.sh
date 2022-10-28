#!/bin/bash -xe

# Usage:
#   $0 $join_arguments...
#
# Assumptions:
#   - microk8s is installed
#   - microk8s node is ready to join the cluster

while ! microk8s join $@; do
  echo "Failed to join MicroK8s cluster, will retry"
  sleep 5
done
