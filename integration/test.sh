#!/bin/bash

set -ex

_DIR="${BASH_SOURCE%/*}"
if [[ ! -d "$_DIR" ]]; then _DIR="$PWD"; fi

while ! kubectl wait --for=condition=available deploy/capi-microk8s-bootstrap-controller-manager -n capi-microk8s-bootstrap-system; do
    echo "Waiting for bootstrap controller to come up"
    sleep 3
done
while ! kubectl wait --for=condition=available deploy/capi-microk8s-control-plane-controller-manager -n capi-microk8s-control-plane-system; do
    echo "Waiting for control-plane controller to come up"
    sleep 3
done

echo "Deploy a test cluster"

# generate cluster
export AWS_REGION=us-east-1
export AWS_SSH_KEY_NAME=capi
export CONTROL_PLANE_MACHINE_COUNT=3
export WORKER_MACHINE_COUNT=3
export AWS_CREATE_BASTION=false
export AWS_PUBLIC_IP=false
export AWS_CONTROL_PLANE_MACHINE_FLAVOR=t3.large
export AWS_NODE_MACHINE_FLAVOR=t3.large
clusterctl generate cluster test-ci-cluster --from "${_DIR}/../templates/cluster-template-aws.yaml" --kubernetes-version 1.25.0 > cluster.yaml

function cleanup() {
    set +e
    kubectl delete cluster test-ci-cluster
}
trap cleanup EXIT

# have AWS infrastructure provider logs running
kubectl logs -n capa-system deploy/capa-controller-manager -f &

# deploy cluster
kubectl apply -f cluster.yaml

# get cluster kubeconfig
while ! clusterctl get kubeconfig test-ci-cluster > ./kubeconfig; do
    kubectl get cluster,awscluster

    echo waiting for workload cluster kubeconfig
    sleep 30
done

# wait for nodes to come up
while ! kubectl --kubeconfig=./kubeconfig get node | grep "\<Ready\>" | wc -l | grep 6; do
    kubectl get cluster,machines,awscluster,awsmachines
    kubectl --kubeconfig=./kubeconfig get node || true

    echo waiting for 6 nodes to become ready
    sleep 20
done

# create deploy and wait for pods
kubectl --kubeconfig=./kubeconfig create deploy --image cdkbot/microbot:1 --replicas 30 bot
while ! kubectl --kubeconfig=./kubeconfig wait deploy/bot --for=jsonpath='{.status.readyReplicas}=30'; do
    kubectl --kubeconfig=./kubeconfig get node,pod -A || true

    echo waiting for deployment to come up
    sleep 20
done
