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
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/util/version"
)

// WorkerInput defines the context needed to generate a worker instance to join a cluster.
type WorkerInput struct {
	// ControlPlaneEndpoint is the control plane endpoint of the cluster.
	ControlPlaneEndpoint string
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
	// JoinNodeIPs is the IP addresses of the nodes to join.
	JoinNodeIPs []string
	// Confinement specifies a classic or strict deployment of microk8s snap.
	Confinement string
	// RiskLevel specifies the risk level (strict, candidate, beta, edge) for the snap channels.
	RiskLevel string
	// SnapstoreProxyScheme specifies the scheme (e.g http or https) of the domain. Defaults to http.
	SnapstoreProxyScheme string
	// SnapstoreProxyDomain specifies the domain of the snapstore proxy if one is to be used.
	SnapstoreProxyDomain string
	// SnapstoreProxyId specifies the snapstore proxy ID if one is to be used.
	SnapstoreProxyId string
	// ExtraWriteFiles is a list of extra files to inject with cloud-init.
	ExtraWriteFiles []File
	// ExtraKubeletArgs is a list of arguments to add to kubelet.
	ExtraKubeletArgs []string
	// SnapstoreHTTPProxy is http_proxy configuration for snap store.
	SnapstoreHTTPProxy string
	// SnapstoreHTTPSProxy is https_proxy configuration for snap store.
	SnapstoreHTTPSProxy string
	// BootCommands is a list of commands to add to the "bootcmd" section of cloud-init.
	BootCommands []string
	// PreRunCommands is a list of commands to add to the "runcmd" section of cloud-init before installing MicroK8s.
	PreRunCommands []string
	// PostRunCommands is a list of commands to add to the "runcmd" section of cloud-init after installing MicroK8s.
	PostRunCommands []string
}

func NewJoinWorker(input *WorkerInput) (*CloudConfig, error) {
	// ensure token is valid
	if len(input.Token) != 32 {
		return nil, fmt.Errorf("join token %q is invalid; length must be 32 characters", input.Token)
	}

	// figure out snap channel from KubernetesVersion
	kubernetesVersion, err := version.ParseSemantic(input.KubernetesVersion)
	if err != nil {
		return nil, fmt.Errorf("kubernetes version %q is not a semantic version: %w", input.KubernetesVersion, err)
	}

	// strict confinement is only available for microk8s v1.25+
	if input.Confinement == "strict" && kubernetesVersion.Minor() < 25 {
		return nil, fmt.Errorf("strict confinement is only available for microk8s v1.25+")
	}

	if input.SnapstoreProxyScheme == "" {
		input.SnapstoreProxyScheme = "http"
	}

	stopApiServerProxyRefreshes := "no"
	if kubernetesVersion.Minor() > 24 {
		stopApiServerProxyRefreshes = "yes"
	}
	installArgs := createInstallArgs(input.Confinement, input.RiskLevel, kubernetesVersion)

	cloudConfig := NewBaseCloudConfig()
	cloudConfig.WriteFiles = append(cloudConfig.WriteFiles, input.ExtraWriteFiles...)
	if args := input.ExtraKubeletArgs; len(args) > 0 {
		cloudConfig.WriteFiles = append(cloudConfig.WriteFiles, File{
			Content:     strings.Join(args, "\n"),
			Path:        filepath.Join("/var", "tmp", "extra-kubelet-args"),
			Permissions: "0400",
			Owner:       "root:root",
		})
	}

	joinURLs := make([]string, 0, len(input.JoinNodeIPs))
	for _, nodeIP := range input.JoinNodeIPs {
		joinURLs = append(joinURLs, fmt.Sprintf("%q", fmt.Sprintf("%s:%s/%s", nodeIP, input.ClusterAgentPort, input.Token)))
	}

	cloudConfig.BootCommands = append(cloudConfig.BootCommands, input.BootCommands...)

	cloudConfig.RunCommands = append(cloudConfig.RunCommands, input.PreRunCommands...)
	cloudConfig.RunCommands = append(cloudConfig.RunCommands,
		fmt.Sprintf("%s %q %q", scriptPath(snapstoreHTTPProxyScript), input.SnapstoreHTTPProxy, input.SnapstoreHTTPSProxy),
		fmt.Sprintf("%s %q %q %q", scriptPath(snapstoreProxyScript), input.SnapstoreProxyScheme, input.SnapstoreProxyDomain, input.SnapstoreProxyId),
		scriptPath(disableHostServicesScript),
		fmt.Sprintf("%s %q %v", scriptPath(installMicroK8sScript), installArgs, false),
		fmt.Sprintf("%s %q %q %q", scriptPath(configureContainerdProxyScript), input.ContainerdHTTPProxy, input.ContainerdHTTPSProxy, input.ContainerdNoProxy),
		scriptPath(configureKubeletScript),
		scriptPath(waitAPIServerScript),
		fmt.Sprintf("%s %q", scriptPath(configureClusterAgentPortScript), input.ClusterAgentPort),
		fmt.Sprintf("%s yes %s", scriptPath(microk8sJoinScript), strings.Join(joinURLs, " ")),
		fmt.Sprintf("%s %s 6443 %s", scriptPath(configureTraefikScript), input.ControlPlaneEndpoint, stopApiServerProxyRefreshes),
	)
	cloudConfig.RunCommands = append(cloudConfig.RunCommands, input.PostRunCommands...)

	return cloudConfig, nil
}
