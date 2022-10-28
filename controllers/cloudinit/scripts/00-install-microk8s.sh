#!/bin/bash -xe

# Usage:
#   $0 $microk8s_snap_channel
#
# Assumptions:
#   - snap is installed

while ! snap install microk8s --classic --channel "${1}"; do
  echo "Failed to install MicroK8s snap, will retry"
  sleep 5
done
