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
- sudo sh -c "while ! snap install microk8s --classic ; do sleep 10 ; echo 'Retry snap installation'; done"
- sudo microk8s status --wait-ready
- sudo sed -i 's/25000/2379/' /var/snap/microk8s/current/args/cluster-agent
- sudo grep Address /var/snap/microk8s/current/var/kubernetes/backend/info.yaml > /var/tmp/port-update.yaml
- sudo sed -i 's/19001/2380/' /var/tmp/port-update.yaml
- sudo microk8s stop
- sudo mv /var/tmp/port-update.yaml /var/snap/microk8s/current/var/kubernetes/backend/update.yaml
- sudo microk8s start
- sudo microk8s status --wait-ready
- sudo sed -i '/^DNS.1 = kubernetes/a {{.ControlPlaneEndpointType}}.100 = {{.ControlPlaneEndpoint}}' /var/snap/microk8s/current/certs/csr.conf.template
- sudo sleep 10
- sudo microk8s status --wait-ready
- sudo sh -c "while ! microk8s join {{.IPOfNodeToJoin}}:{{.PortOfNodeToJoin}}/{{.JoinToken}} ; do sleep 10 ; echo 'Retry join'; done"
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
	Version                  string
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
