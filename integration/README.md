## Integration tests

### Prerequisites

The integration/e2e tests have the following prerequisites:

  * an environment variable `CLUSTER_MANIFEST_FILE` pointing to the cluster manifest. Cluster manifests can be produced with the help of the templates found under `templates`. For example:
  ```
    export AWS_REGION=us-east-1
    export AWS_SSH_KEY_NAME=capi
    export CONTROL_PLANE_MACHINE_COUNT=3
    export WORKER_MACHINE_COUNT=3
    export AWS_CREATE_BASTION=false
    export AWS_PUBLIC_IP=false
    export AWS_CONTROL_PLANE_MACHINE_FLAVOR=t3.large
    export AWS_NODE_MACHINE_FLAVOR=t3.large
    clusterctl generate cluster test-ci-cluster --from "bootstrap/templates/cluster-template-aws.yaml" --kubernetes-version 1.25.0 > cluster.yaml
    export CLUSTER_MANIFEST_FILE=$PWD/cluster.yaml
  ```

  * `clusterctl` available in the PATH

  * `kubectl` available in the PATH

  * a management cluster initialised via `clusterctl` with the infrastructure targeted as well as the version of the MicroK8s providers we want to be tested

  * the `kubeconfig` of the management cluster in the default location `$HOME/.kube/config`


### Running the tests

Just execute the `test.sh`.
