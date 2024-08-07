#!/bin/bash -xe

# Usage:
#   $0 $snapstore-scheme $snapstore-domain $snapstore-id
#
# Arguments:
#   $snapstore-scheme     The scheme for the domain (e.g. https or http without the ://)
#   $snapstore-domain     The domain name (e.g. snapstore.domain.com)
#   $snapstore-id         The store id (e.g. ID123456789)
#
# Assumptions:
#   - snapd is installed

if [ "$#" -ne 3 ] || [ -z "${1}" ] || [ -z "${2}" ] || [ -z "${3}" ] ; then
  echo "Using the default snapstore"
  exit 0
fi

if ! type -P curl ; then
  while ! snap install curl; do
    echo "Failed to install curl, will retry"
    sleep 5
  done
fi

while ! curl -sL "${1}"://"${2}"/v2/auth/store/assertions | snap ack /dev/stdin ; do
  echo "Failed to ACK store assertions, will retry"
  sleep 5
done

while ! snap set core proxy.store="${3}" ; do
  echo "Failed to configure snapd with store ID, will retry"
  sleep 5
done

systemctl restart snapd
