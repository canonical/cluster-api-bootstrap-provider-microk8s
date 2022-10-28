#!/bin/bash -xe

# Usage:
#   $0 $new_dqlite_port
#
# Assumptions:
#   - microk8s is installed
#   - dqlite has been initialized on the node and is running

DQLITE="/var/snap/microk8s/current/var/kubernetes/backend"

grep "Address" "${DQLITE}/info.yaml" | sed "s/19001/${1}/" | tee "${DQLITE}/update.yaml"

snap restart microk8s.daemon-k8s-dqlite
