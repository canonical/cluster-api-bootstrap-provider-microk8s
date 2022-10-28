#!/bin/bash -xe

# Usage:
#   $0
#
# Assumptions:
#   - systemctl is available

for svc in kubelet containerd; do
  systemctl stop "${svc}" || true
  systemctl disable "${svc}" || true
done
