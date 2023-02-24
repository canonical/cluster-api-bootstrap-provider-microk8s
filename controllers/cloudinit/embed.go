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
	"embed"
	"fmt"
	"path/filepath"
)

var (
	//go:embed scripts
	embeddedScripts embed.FS
)

// script is a type-alias used to ensure we do not have issues with script names.
type script string

const (
	// cloudConfigTemplate is the template to render the cloud-config for the instances.
	cloudConfigTemplate script = "cloud-config-template"

	// snapstoreProxyScript configures a snapstore proxy.
	snapstoreProxyScript script = "00-configure-snapstore-proxy.sh"

	// disableHostServicesScript disables services like containerd or kubelet from the host OS image.
	disableHostServicesScript script = "00-disable-host-services.sh"

	// installMicroK8sScript installs MicroK8s on the host.
	installMicroK8sScript script = "00-install-microk8s.sh"

	// configureCertLB configures the server certificate so it is valid for the LB.
	configureCertLB script = "10-configure-cert-for-lb.sh"

	// configureAPIServerScript configures arguments and sets apiserver port to 6443.
	configureAPIServerScript script = "10-configure-apiserver.sh"

	// configureCalicoIPIPScript configures Calico to use IPinIP.
	configureCalicoIPIPScript script = "10-configure-calico-ipip.sh"

	// configureClusterAgentPortScript configures the port of the cluster agent.
	configureClusterAgentPortScript script = "10-configure-cluster-agent-port.sh"

	// configureContainerdProxyScript configures proxy settings for containerd.
	configureContainerdProxyScript script = "10-configure-containerd-proxy.sh"

	// configureDqlitePortScript configures the port used by dqlite.
	configureDqlitePortScript script = "10-configure-dqlite-port.sh"

	// configureKubeletScript configures the kubelet.
	configureKubeletScript script = "10-configure-kubelet.sh"

	// microk8sEnableScript enables MicroK8s addons.
	microk8sEnableScript script = "20-microk8s-enable.sh"

	// microk8sJoinScript joins the current node to a MicroK8s cluster.
	microk8sJoinScript script = "20-microk8s-join.sh"

	// configureTraefikScript configures the control plane endpoint in the traefik provider configuration.
	configureTraefikScript script = "30-configure-traefik.sh"
)

var allScripts = []script{
	snapstoreProxyScript,
	disableHostServicesScript,
	installMicroK8sScript,
	configureCertLB,
	configureAPIServerScript,
	configureCalicoIPIPScript,
	configureClusterAgentPortScript,
	configureContainerdProxyScript,
	configureDqlitePortScript,
	configureTraefikScript,
	configureKubeletScript,
	microk8sEnableScript,
	microk8sJoinScript,
}

func mustGetScript(scriptName script) string {
	b, err := embeddedScripts.ReadFile(filepath.Join("scripts", string(scriptName)))
	if err != nil {
		panic(fmt.Errorf("missing embedded script %v", scriptName))
	}
	return string(b)
}

func scriptPath(scriptName script) string {
	return filepath.Join("/capi-scripts", string(scriptName))
}
