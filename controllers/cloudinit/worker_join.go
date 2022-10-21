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
	"strings"

	"github.com/pkg/errors"
)

const (
	workerJoinCloudInit = `{{.Header}}
runcmd:
- sudo echo IPOfNodeToJoin {{.IPOfNodeToJoin}}
- sudo echo PortOfNodeToJoin {{.PortOfNodeToJoin}}
- sudo echo Version {{.Version}}
- sudo systemctl stop kubelet || true
- sudo systemctl disable kubelet || true
- sudo systemctl stop containerd || true
- sudo systemctl disable containerd || true
- sudo sh -c "while ! snap install microk8s --classic {{.Version}} ; do sleep 10 ; echo 'Retry snap installation'; done"
- sudo microk8s status --wait-ready
- sudo echo "Stopping"
- sudo microk8s stop
- sudo sleep 20
{{.ProxySection}}
- sudo sed -i 's/25000/{{.PortOfNodeToJoin}}/' /var/snap/microk8s/current/args/cluster-agent
- sudo echo "Starting"
- sudo microk8s start
- sudo sleep 20
- sudo microk8s status --wait-ready
- sudo echo "Joining"
- sudo echo "Will join {{.IPOfNodeToJoin}}:{{.PortOfNodeToJoin}}"
- sudo sh -c "while ! microk8s join {{.IPOfNodeToJoin}}:{{.PortOfNodeToJoin}}/{{.JoinToken}} --worker ; do sleep 10 ; echo 'Retry join'; done"
- sudo  sed -i '/.*address:.*/d' /var/snap/microk8s/current/args/traefik/provider.yaml
- |
  sudo echo "        - address: {{.ControlPlaneEndpoint}}:6443" >> /var/snap/microk8s/current/args/traefik/provider.yaml
`
)

// WorkerJoinInput defines context to generate instance user data for worker nodes to join.
type WorkerJoinInput struct {
	BaseUserData
	JoinToken            string
	IPOfNodeToJoin       string
	PortOfNodeToJoin     string
	Version              string
	ControlPlaneEndpoint string
	HTTPSProxy           *string
	HTTPProxy            *string
	NoProxy              *string
}

// NewJoinWorker returns the user data string to be used on a new worker instance.
func NewJoinWorker(input *WorkerJoinInput) ([]byte, error) {
	input.Header = cloudConfigHeader
	major, minor, err := extractVersionParts(input.Version)
	if err != nil {
		return nil, err
	}
	input.Version = generateSnapChannelArgument(major, minor)

	proxyCommands := generateProxyCommands(input.HTTPSProxy, input.HTTPProxy, input.NoProxy)
	cloudinitStr := strings.Replace(workerJoinCloudInit, "{{.ProxySection}}", proxyCommands, -1)

	userData, err := generate("JoinWorker", cloudinitStr, input)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate user data for machine joining as worker")
	}

	return userData, err
}
