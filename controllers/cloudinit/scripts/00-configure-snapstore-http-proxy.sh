#!/bin/bash -xe

# Usage:
#   $0 $snapstore-http-proxy $snapstore-https-proxy
#
# Assumptions:
#   - snapd is installed

if [[ "${1}" != "" ]]; then
  snap set system proxy.http="${1}"
fi

if [[ "${2}" != "" ]]; then
  snap set system proxy.https="${2}"
fi
