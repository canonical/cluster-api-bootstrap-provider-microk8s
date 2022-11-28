#!/bin/bash -xe

# Usage:
#   $0
#
# Assumptions:
#   - microk8s is installed
#   - /var/tmp/extra-kubelet-args exists

EXTRA_ARGS_FILE="/var/tmp/extra-kubelet-args"
KUBELET_ARGS="/var/snap/microk8s/current/args/kubelet"

if [ ! -f "${EXTRA_ARGS_FILE}" ]; then
  echo "No extra kubelet configuration needed"
  exit 0
if

echo "" >> "${KUBELET_ARGS}"
echo "# ClusterAPI configuration" >> "${KUBELET_ARGS}"
cat "${EXTRA_ARGS_FILE}" >> "${KUBELET_ARGS}"

# restart kubelite so that kubelet picks up the new arguments
snap restart microk8s.daemon-kubelite
