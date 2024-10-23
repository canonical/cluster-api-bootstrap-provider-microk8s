#!/bin/bash -xe

# Usage:
#   $0 $worker_yes_no $join_string_1 $join_string_2 ... $join_string_N
#
# Assumptions:
#   - microk8s is installed
#   - microk8s node is ready to join the cluster

join_args=""
if [ ${1} == "yes" ]; then
  join_args="--worker"
fi

shift

# Loop over the given join addresses until microk8s join command succeeds.
joined="false"
attempts=0
max_attempts=30
while [ "$joined" = "false" ]; do

  for url in "${@}"; do
    if [ $attempts -ge $max_attempts ]; then
      echo "Max join retry limit reached, exiting."
      exit 1
    fi

    if microk8s join "${url}" $join_args; then
      joined="true"
      break
    fi

    echo "Failed to join MicroK8s cluster, retrying ($((attempts+1))/$max_attempts)"
    attempts=$((attempts+1))
    sleep 5
  done

done

# What is this hack? Why do we call snap set here?
# "snap set microk8s ..." will call the configure hook.
# The configure hook is where we sanitise arguments to k8s services.
# When we join a node to a cluster the arguments of kubelet/api-server
# are copied from the "control plane" node to the joining node.
# It is possible some deprecated/removed arguments are copied over.
# For example if we join a 1.24 node to 1.23 cluster arguments like
# --network-plugin will cause kubelite to crashloop.
# Threfore we call the conigure hook to clean things.
# PS. This should be a workaround to a MicroK8s bug.
while ! snap set microk8s configure=call$$; do
  echo "Failed to call the configure hook, will retry"
  sleep 5
done
sleep 10

while ! snap restart microk8s.daemon-containerd; do
  sleep 5
done
while ! snap restart microk8s.daemon-kubelite; do
  sleep 5
done
sleep 10

if [ ${1} == "no" ]; then
  /capi-scripts/50-wait-apiserver.sh
fi
