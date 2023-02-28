#!/bin/bash -xe

# Usage:
#   $0 $snapstore-domain $snapstore-id
#
# Assumptions:
#   - snapd is installed

if [ "$#" -ne 2 ] || [ -z "${1}" ] || [ -z "${2}" ] ; then
  echo "Using the default snapstore"
  exit 0
fi

if ! type -P curl ; then
  while ! snap install curl; do
    echo "Failed to install curl, will retry"
    sleep 5
  done
fi

while ! curl -sL http://"${1}"/v2/auth/store/assertions | snap ack /dev/stdin ; do
  echo "Failed to ACK store assertions, will retry"
  sleep 5
done

while ! snap set core proxy.store="${2}" ; do
  echo "Failed to configure snapd with stire ID, will retry"
  sleep 5
done
