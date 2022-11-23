/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudinit

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/version"
)

// WorkerInput defines the context needed to generate a worker instance to join a cluster.
type WorkerInput struct {
	// Token is the token that will be used for joining other nodes to the cluster.
	Token string
	// KubernetesVersion is the Kubernetes version we want to install.
	KubernetesVersion string
	// ClusterAgentPort is the port that cluster-agent binds to.
	ClusterAgentPort string
	// ContainerdHTTPProxy is http_proxy configuration for containerd.
	ContainerdHTTPProxy string
	// ContainerdHTTPSProxy is https_proxy configuration for containerd.
	ContainerdHTTPSProxy string
	// ContainerdNoProxy is no_proxy configuration for containerd.
	ContainerdNoProxy string
	// JoinNodeIP is the IP address of the node to join
	JoinNodeIP string
	// Confinement specifies a classic or strict deployment of microk8s snap
	Confinement string
}

func NewJoinWorker(input *WorkerInput) (*CloudConfig, error) {
	// ensure token is valid
	if len(input.Token) != 32 {
		return nil, fmt.Errorf("join token %q is invalid; length must be 32 characters", input.Token)
	}

	// figure out snap channel from KubernetesVersion
	// TODO: support specifying the snap channel
	kubernetesVersion, err := version.ParseSemantic(input.KubernetesVersion)
	if err != nil {
		return nil, fmt.Errorf("kubernetes version %q is not a semantic version: %w", input.KubernetesVersion, err)
	}

	cloudConfig := NewBaseCloudConfig()

	cloudConfig.RunCommands = append(cloudConfig.RunCommands,
		"set -x",
		scriptPath(disableHostServicesScript),
		fmt.Sprintf("%s %d.%d %s", scriptPath(installMicroK8sScript), kubernetesVersion.Major(), kubernetesVersion.Minor(), input.Confinement),
		fmt.Sprintf("%s %q %q %q", scriptPath(configureContainerdProxyScript), input.ContainerdHTTPProxy, input.ContainerdHTTPSProxy, input.ContainerdNoProxy),
		"microk8s status --wait-ready",
		fmt.Sprintf("%s %q", scriptPath(configureClusterAgentPortScript), input.ClusterAgentPort),
		fmt.Sprintf("%s %q --worker", scriptPath(microk8sJoinScript), fmt.Sprintf("%s:%s/%s", input.JoinNodeIP, input.ClusterAgentPort, input.Token)),
	)

	return cloudConfig, nil
}
