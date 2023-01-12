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
# allow for some time for the k8s services to start and
# also to have the apiserver-proxy fetch the new endpoint
# when/if this is a CP node
sleep 35