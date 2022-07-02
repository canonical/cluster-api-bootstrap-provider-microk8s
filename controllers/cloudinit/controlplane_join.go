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

	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api/util/secret"
)

const (
	controlPlaneJoinCloudInit = `{{.Header}}
runcmd:
- sudo iptables -t nat -A OUTPUT -o lo -p tcp --dport 6443 -j REDIRECT --to-port 16443
- sudo iptables -A PREROUTING -t nat  -p tcp --dport 6443 -j REDIRECT --to-port 16443
- sudo apt-get update
- sudo apt-get install iptables-persistent
- sudo snap install microk8s --classic
- sudo microk8s status --wait-ready
- sudo sed -i '/^DNS.1 = kubernetes/a {{.ControlPlaneEndpointType}}.100 = {{.ControlPlaneEndpoint}}' /var/snap/microk8s/current/certs/csr.conf.template
- sudo sleep 10
- sudo microk8s status --wait-ready
- sudo microk8s join {{.IPOfNodeToJoin}}:{{.PortOfNodeToJoin}}/{{.JoinToken}}
- sudo sleep 20
- sudo microk8s status --wait-ready
- sudo microk8s add-node --token-ttl 86400 --token {{.JoinToken}}
`
)

// ControlPlaneJoinInput defines context to generate controlplane instance user data for control plane node join.
type ControlPlaneJoinInput struct {
	BaseUserData
	secret.Certificates
	ControlPlaneEndpoint     string
	ControlPlaneEndpointType string
	JoinToken                string
	IPOfNodeToJoin           string
	PortOfNodeToJoin         string
}

// NewJoinControlPlane returns the user data string to be used on a new control plane instance.
func NewJoinControlPlane(input *ControlPlaneJoinInput) ([]byte, error) {
	// TODO: Consider validating that the correct certificates exist. It is different for external/stacked etcd
	input.WriteFiles = input.Certificates.AsFiles()
	input.ControlPlane = true
	if err := input.prepare(); err != nil {
		return nil, err
	}
	input.ControlPlaneEndpointType = "DNS"
	addr := net.ParseIP(input.ControlPlaneEndpoint)
	if addr != nil {
		input.ControlPlaneEndpointType = "IP"
	}
	userData, err := generate("JoinControlplane", controlPlaneJoinCloudInit, input)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate user data for machine joining control plane")
	}

	return userData, err
}
