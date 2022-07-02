## Cluster API bootstrap provider for MicroK8s

[Cluster API](https://cluster-api.sigs.k8s.io/) provides declarative APIs to provision, upgrade, and operate Kubernetes clusters.

The [bootstrap provider controller in cluster API](https://cluster-api.sigs.k8s.io/user/concepts.html#bootstrap-provider) is responsible for initializing the control plane and worker nodes of the provisioned cluster.

This project offers a cluster API bootstrap provider controller that manages the node provision of a [MicroK8s](https://github.com/canonical/microk8s) cluster. It is expected to be used along with the respective [MicroK8s specific control plane provider](https://github.com/canonical/cluster-api-control-plane-provider-microk8s).


![arch](images/arch.png)


# Development

  * Install clusterctl following the [upstream instructions](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl)
```
curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.1.3/clusterctl-linux-amd64 -o clusterctl
```

  * Install a MicroK8s bootstrap cluster
```
sudo snap install microk8s --classic
sudo microk8s.config  > ~/.kube/config
sudo microk8s enable dns
```

  * Install the cluster provider of your choice. Have a look at the [cluster API book](https://cluster-api.sigs.k8s.io/user/quick-start.html#initialization-for-common-providers) for your options at this step. You should deploy only the infrastructure controller leaving the bootstrap and control plane ones empty. For example assuming we want to provision a MicroK8s cluster on OpenStack:
```
clusterctl init --infrastructure openstack --bootstrap "-" --control-plane "-"
``` 

  * Clone the two cluster API MicroK8s specific repositories and start the controllers on two separate terminals:
```
cd $GOPATH/src/github.com/canonical/cluster-api-bootstrap-provider-microk8s/ 
make install
make run
``` 
And:
```
cd $GOPATH/src/github.com/canonical/cluster-api-control-plane-provider-microk8s/ 
make install
make run
``` 

  * Apply the cluster manifests describing the desired specs of the cluster you want to provision.