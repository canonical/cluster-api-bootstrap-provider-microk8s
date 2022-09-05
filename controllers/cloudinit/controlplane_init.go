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
)

/*
The following cloudinit includes a number of hacks we need to address as we move forward:

 - redirect the API server port from 16443 to 6443
By default MicroK8s sets the API server port to 16443. We should investigate two options here:
1. See how we can configure the security groups of the infra providers to allow 16443.
2. Get the API server port configured to 6443

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
- sudo sed -i 's/25000/{{.PortOfClusterAgent}}/' /var/snap/microk8s/current/args/cluster-agent
- sudo grep Address /var/snap/microk8s/current/var/kubernetes/backend/info.yaml > /var/tmp/port-update.yaml
- sudo sed -i 's/19001/{{.PortOfDqlite}}/' /var/tmp/port-update.yaml
- sudo microk8s stop
{{.ProxySection}}
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
	CACert                   string
	CAKey                    string
	ControlPlaneEndpoint     string
	ControlPlaneEndpointType string
	JoinToken                string
	JoinTokenTTLInSecs       int64
	Version                  string
	PortOfClusterAgent       string
	PortOfDqlite             string
	HTTPSProxy               *string
	HTTPProxy                *string
	NoProxy                  *string
	Addons                   []string
}

// NewInitControlPlane returns the user data string to be used on a controlplane instance.
func NewInitControlPlane(input *ControlPlaneInput) ([]byte, error) {
	input.Header = cloudConfigHeader
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

	var addonsStr string
	for _, addon := range input.Addons {
		addonsStr += fmt.Sprintf(" '%s' ", addon)
	}
	cloudinitStr := strings.Replace(controlPlaneCloudInit, "{{.Addons}}", addonsStr, -1)

	proxyCommands := generateProxyCommands(input.HTTPSProxy, input.HTTPProxy, input.NoProxy)
	cloudinitStr = strings.Replace(cloudinitStr, "{{.ProxySection}}", proxyCommands, -1)

	addr := net.ParseIP(input.ControlPlaneEndpoint)
	if addr != nil {
		input.ControlPlaneEndpointType = "IP"
	}

	userData, err := generate("InitControlplane", cloudinitStr, input)
	if err != nil {
		return nil, err
	}

	return userData, nil
}
