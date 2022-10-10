/*
Copyright 2019 The Kubernetes Authors.

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
	"net"
	"strings"

	"github.com/pkg/errors"
)

const (
	controlPlaneJoinCloudInit = `{{.Header}}
runcmd:
- sudo echo ControlPlaneEndpoint {{.ControlPlaneEndpoint}}
- sudo echo ControlPlaneEndpointType {{.ControlPlaneEndpointType}}
- sudo echo JoinTokenTTLInSecs {{.JoinTokenTTLInSecs}}
- sudo echo IPOfNodeToJoin {{.IPOfNodeToJoin}}
- sudo echo PortOfNodeToJoin {{.PortOfNodeToJoin}}
- sudo echo Version {{.Version}}
- sudo sh -c "while ! snap install microk8s --classic {{.Version}}; do sleep 10 ; echo 'Retry snap installation'; done"
- sudo microk8s status --wait-ready
- sudo echo "--service-node-port-range=30001-32767" >> /var/snap/microk8s/current/args/kube-apiserver
{{.IPinIPSection}}
- sudo microk8s stop
- sudo sed -i 's/25000/{{.PortOfNodeToJoin}}/' /var/snap/microk8s/current/args/cluster-agent
- sudo grep Address /var/snap/microk8s/current/var/kubernetes/backend/info.yaml > /var/tmp/port-update.yaml
- sudo sed -i 's/19001/{{.PortOfDqlite}}/' /var/tmp/port-update.yaml
- |
  sudo sed 's/CALICO_IPV4POOL_VXLAN/CALICO_IPV4POOL_IPIP/' -i /var/snap/microk8s/current/args/cni-network/cni.yaml
  sudo sed 's/calico_backend: "vxlan"/calico_backend: "bird"/' -i /var/snap/microk8s/current/args/cni-network/cni.yaml
  sudo sed -i 's/ *# - -bird-ready/                - -bird-ready/' /var/snap/microk8s/current/args/cni-network/cni.yaml
  sudo sed -i 's/ *# - -bird-live/                - -bird-live/' /var/snap/microk8s/current/args/cni-network/cni.yaml
{{.ProxySection}}
- sudo mv /var/tmp/port-update.yaml /var/snap/microk8s/current/var/kubernetes/backend/update.yaml
- sudo microk8s start
- sudo microk8s status --wait-ready
- sudo sed -i '/^DNS.1 = kubernetes/a {{.ControlPlaneEndpointType}}.100 = {{.ControlPlaneEndpoint}}' /var/snap/microk8s/current/certs/csr.conf.template
- sudo sleep 10
- sudo microk8s status --wait-ready
- sudo sh -c "while ! microk8s join {{.IPOfNodeToJoin}}:{{.PortOfNodeToJoin}}/{{.JoinToken}} ; do sleep 10 ; echo 'Retry join'; done"
- sudo sleep 20
- sudo microk8s status --wait-ready
- sudo microk8s stop
- sudo iptables -t nat -A OUTPUT -o lo -p tcp --dport 16443 -j REDIRECT --to-port 6443
- sudo iptables -A PREROUTING -t nat  -p tcp --dport 16443 -j REDIRECT --to-port 6443
- sudo apt-get update
- sudo apt-get install iptables-persistent
- sudo sed -i 's/16443/6443/' /var/snap/microk8s/current/args/kube-apiserver
- sudo sed -i 's/16443/6443/' /var/snap/microk8s/current/credentials/client.config
- sudo sed -i 's/16443/6443/' /var/snap/microk8s/current/credentials/scheduler.config
- sudo sed -i 's/16443/6443/' /var/snap/microk8s/current/credentials/kubelet.config
- sudo sed -i 's/16443/6443/' /var/snap/microk8s/current/credentials/proxy.config
- sudo sed -i 's/16443/6443/' /var/snap/microk8s/current/credentials/controller.config
- sudo microk8s start
- sudo microk8s status --wait-ready
- sudo microk8s add-node --token-ttl {{.JoinTokenTTLInSecs}} --token {{.JoinToken}}
`
)

// ControlPlaneJoinInput defines context to generate controlplane instance user data for control plane node join.
type ControlPlaneJoinInput struct {
	BaseUserData
	ControlPlaneEndpoint     string
	ControlPlaneEndpointType string
	JoinToken                string
	JoinTokenTTLInSecs       int64
	IPOfNodeToJoin           string
	PortOfNodeToJoin         string
	PortOfDqlite             string
	Version                  string
	HTTPSProxy               *string
	HTTPProxy                *string
	NoProxy                  *string
	IPinIP                   bool
}

// NewJoinControlPlane returns the user data string to be used on a new control plane instance.
func NewJoinControlPlane(input *ControlPlaneJoinInput) ([]byte, error) {
	input.Header = cloudConfigHeader
	major, minor, err := extractVersionParts(input.Version)
	if err != nil {
		return nil, err
	}
	input.Version = generateSnapChannelArgument(major, minor)

	input.ControlPlaneEndpointType = "DNS"
	addr := net.ParseIP(input.ControlPlaneEndpoint)
	if addr != nil {
		input.ControlPlaneEndpointType = "IP"
	}

	proxyCommands := generateProxyCommands(input.HTTPSProxy, input.HTTPProxy, input.NoProxy)
	cloudinitStr := strings.Replace(controlPlaneJoinCloudInit, "{{.ProxySection}}", proxyCommands, -1)

	ipinipCommands := generateIPinIPCommands(input.IPinIP)
	cloudinitStr = strings.Replace(cloudinitStr, "{{.IPinIPSection}}", ipinipCommands, -1)

	userData, err := generate("JoinControlplane", cloudinitStr, input)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate user data for machine joining control plane")
	}

	return userData, err
}
