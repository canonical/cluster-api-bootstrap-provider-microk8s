#!/bin/bash -xe

# Usage:
#   $0
#
# Assumptions:
#   - microk8s is installed
#   - iptables is installed
#   - apt is available for installing packages

APISERVER_ARGS="${APISERVER_ARGS:-/var/snap/microk8s/current/args/kube-apiserver}"
CREDENTIALS_DIR="${CREDENTIALS_DIR:-/var/snap/microk8s/current/credentials}"

# Configure command-line arguments for kube-apiserver
echo "
--service-node-port-range=30001-32767
" >> "${APISERVER_ARGS}"

# Configure apiserver port
sed 's/16443/6443/' -i "${APISERVER_ARGS}"

# Configure apiserver port for service config files
sed 's/16443/6443/' -i "${CREDENTIALS_DIR}/client.config"
sed 's/16443/6443/' -i "${CREDENTIALS_DIR}/scheduler.config"
sed 's/16443/6443/' -i "${CREDENTIALS_DIR}/kubelet.config"
sed 's/16443/6443/' -i "${CREDENTIALS_DIR}/proxy.config"
sed 's/16443/6443/' -i "${CREDENTIALS_DIR}/controller.config"

while ! snap set microk8s hack.update.csr=call$$; do
  echo "Failed to call the configure hook, will retry"
  sleep 5
done
sleep 10

while ! snap restart microk8s.daemon-kubelite; do
  sleep 5
done

# delete kubernetes service to make sure port is updated
microk8s status --wait-ready
microk8s kubectl delete svc kubernetes

# redirect port 16443 to 6443
iptables -t nat -A OUTPUT -o lo -p tcp --dport 16443 -j REDIRECT --to-port 6443
iptables -t nat -A PREROUTING   -p tcp --dport 16443 -j REDIRECT --to-port 6443

# ensure rules persist across reboots
apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install iptables-persistent -y
