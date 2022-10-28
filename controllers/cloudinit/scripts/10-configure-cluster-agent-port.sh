#!/bin/bash -xe

# Usage:
#   $0 $new_cluster_agent_port
#
# Assumptions:
#   - microk8s is installed

sed "s/25000/${1}/" -i "/var/snap/microk8s/current/args/cluster-agent"

snap restart microk8s.daemon-cluster-agent
