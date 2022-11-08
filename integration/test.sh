#!/bin/bash

set -ex

if [[ -z "${CLUSTER_MANIFEST_FILE}" ]]; then
    echo "Environemnt variable CLUSTER_MANIFEST_FILE is not set."
    echo "CLUSTER_MANIFEST_FILE is expected to hold the PATH to a cluster manifest."
    exit 1
fi

while ! kubectl wait --for=condition=available deploy/capi-microk8s-bootstrap-controller-manager -n capi-microk8s-bootstrap-system; do
    echo "Waiting for bootstrap controller to come up"
    sleep 3
done
while ! kubectl wait --for=condition=available deploy/capi-microk8s-control-plane-controller-manager -n capi-microk8s-control-plane-system; do
    echo "Waiting for control-plane controller to come up"
    sleep 3
done

echo "Deploy a test cluster"

function cleanup() {
    set +e
    kubectl delete cluster test-ci-cluster
}
trap cleanup EXIT

# deploy cluster
kubectl apply -f ${CLUSTER_MANIFEST_FILE}

# get cluster kubeconfig
while ! clusterctl get kubeconfig test-ci-cluster > ./kubeconfig; do
    kubectl get cluster,awscluster

    echo waiting for workload cluster kubeconfig
    sleep 30
done

echo "Workload cluster kubeconfig file is: "
cat ./kubeconfig

# wait for nodes to come up
while ! kubectl --kubeconfig=./kubeconfig get node | grep "Ready" | wc -l | grep 6; do
    kubectl get cluster,machines || true
    kubectl --kubeconfig=./kubeconfig get node,pod -A || true

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

kubectl --kubeconfig=./kubeconfig get deploy,node,pod -A || true
