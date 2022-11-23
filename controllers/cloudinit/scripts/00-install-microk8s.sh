#!/bin/bash -xe

# Usage:
#   $0 $microk8s_snap_channel $confinement
#
# Assumptions:
#   - snap is installed

install_cmd="snap install microk8s --channel"

if [[ "${2}" == "strict" ]]; then
  install_cmd="$install_cmd "${1}"-strict"
else
  install_cmd="$install_cmd "${1}" --classic"
fi

while ! $install_cmd; do
  echo "Failed to install MicroK8s snap, will retry"
  sleep 5
done
