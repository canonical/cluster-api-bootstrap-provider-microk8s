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
	"fmt"
	"net"
	"strings"

	"sigs.k8s.io/cluster-api/util/secret"
)

/*
The following cloudinit includes a number of hacks we need to address as we move forward:

 - hardcoded CA under /var/tmp/
A single CA is needed to run the cluster. We need to inject a predefined/known CA
because we need to craft a kubeconfig file to access the cluster. If we were to use the CA
MicroK8s creates during snap install we would not know what that CA would be and thus we would not
be able to produce the kubeconfig file.
Instead of having the CA hardcoded we should allow the user to point to an existing secret
with the required cert/key. If the user does not point to a secret we should generate a CA inside the
cluster-api-bootstrap provider and use that one.

 - redirect the API server port from 16443 to 6443
By default MicroK8s sets the API server port to 16443. We should investigate two options here:
1. See how we can configure the security groups of the infra providers to allow 16443.
2. Get the API server port configured to 6443

 - cluster agent port (25000) and dqlite port (19001) set to use etcd ports (2379, 2380)
The default ports of cluster agent and dqlite are blocked by security groups and as a temporary
workaround we reuse the etcd ports that are open in the infra providers we are testing with.
We have to find a way to get the security groups (setup by the infra providers) to allow custom ports.

 - Token for joining nodes
We inject a token for joining nodes. At this point this token is hardcoded with a long TTL.
To the very least we should make the TTL and tokens configurable. Ideally, there would be a mechanism
to join a node without the need of such token.

 - This cloudinit (including the hacks) is somewhat duplicated for the joining nodes we should
address this.
*/

const (
	controlPlaneCloudInit = `{{.Header}}
write_files:
- content: |
{{.CAKey | Indent 4}}
  path: /var/tmp/ca.key
  permissions: '0600'
- content: |
{{.CACert | Indent 4}}
  path: /var/tmp/ca.crt
  permissions: '0600'
runcmd:
- sudo echo ControlPlaneEndpoint {{.ControlPlaneEndpoint}}
- sudo echo ControlPlaneEndpointType {{.ControlPlaneEndpointType}}
- sudo echo JoinTokenTTLInSecs {{.JoinTokenTTLInSecs}}
- sudo echo Version {{.Version}}
- sudo iptables -t nat -A OUTPUT -o lo -p tcp --dport 6443 -j REDIRECT --to-port 16443
- sudo iptables -A PREROUTING -t nat  -p tcp --dport 6443 -j REDIRECT --to-port 16443
- sudo apt-get update
- sudo apt-get install iptables-persistent
- sudo sh -c "while ! snap install microk8s --classic {{.Version}} ; do sleep 10 ; echo 'Retry snap installation'; done"
- sudo sed -i 's/25000/2379/' /var/snap/microk8s/current/args/cluster-agent
- sudo grep Address /var/snap/microk8s/current/var/kubernetes/backend/info.yaml > /var/tmp/port-update.yaml
- sudo sed -i 's/19001/2380/' /var/tmp/port-update.yaml
- sudo microk8s stop
- sudo mv /var/tmp/port-update.yaml /var/snap/microk8s/current/var/kubernetes/backend/update.yaml
- sudo microk8s start
- sudo microk8s status --wait-ready
- sudo microk8s refresh-certs /var/tmp
- sudo sleep 30
- sudo sed -i '/^DNS.1 = kubernetes/a {{.ControlPlaneEndpointType}}.100 = {{.ControlPlaneEndpoint}}' /var/snap/microk8s/current/certs/csr.conf.template
- sudo microk8s status --wait-ready
- sudo microk8s add-node --token-ttl {{.JoinTokenTTLInSecs}} --token {{.JoinToken}}
- sudo sh -c "for a in {{.Addons}} ; do echo 'Enabling ' \$a ; microk8s enable \$a ; sleep 10; microk8s status --wait-ready ; done"
- sudo sleep 15
`
)

// ControlPlaneInput defines the context to generate a controlplane instance user data.
type ControlPlaneInput struct {
	BaseUserData
	secret.Certificates
	CACert                   string
	CAKey                    string
	ControlPlaneEndpoint     string
	ControlPlaneEndpointType string
	JoinToken                string
	JoinTokenTTLInSecs       int64
	Version                  string
	Addons                   []string
}

// NewInitControlPlane returns the user data string to be used on a controlplane instance.
func NewInitControlPlane(input *ControlPlaneInput) ([]byte, error) {
	input.Header = cloudConfigHeader
	input.WriteFiles = input.Certificates.AsFiles()
	input.WriteFiles = append(input.WriteFiles, input.AdditionalFiles...)
	input.SentinelFileCommand = sentinelFileCommand
	input.ControlPlaneEndpointType = "DNS"
	major, minor, err := extractVersionParts(input.Version)
	if err != nil {
		return nil, err
	}
	input.Version = generateSnapChannelArgument(major, minor)

	// Get at least dns enabled
	if input.Addons == nil {
		input.Addons = []string{"dns"}
	}
	found := false
	for _, addon := range input.Addons {
		if strings.Contains(addon, "dns") {
			found = true
			break
		}
	}
	if !found {
		input.Addons = append(input.Addons, "dns")
	}

	var addons_str string
	for _, addon := range input.Addons {
		addons_str += fmt.Sprintf(" '%s' ", addon)
	}
	cloudinit_str := strings.Replace(controlPlaneCloudInit, "{{.Addons}}", addons_str, -1)

	addr := net.ParseIP(input.ControlPlaneEndpoint)
	if addr != nil {
		input.ControlPlaneEndpointType = "IP"
	}

	userData, err := generate("InitControlplane", cloudinit_str, input)
	if err != nil {
		return nil, err
	}

	return userData, nil
}
