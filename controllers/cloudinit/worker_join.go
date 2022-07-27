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
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api/util/secret"
)

const (
	workerJoinCloudInit = `{{.Header}}
runcmd:
- sudo sh -c "while ! snap install microk8s --classic {{.Version}} ; do sleep 10 ; echo 'Retry snap installation'; done"
- sudo microk8s status --wait-ready
- sudo echo "Stopping"
- sudo microk8s stop
- sudo sleep 20
- sudo sed -i 's/25000/2379/' /var/snap/microk8s/current/args/cluster-agent
- sudo echo "Starting"
- sudo microk8s start
- sudo sleep 20
- sudo microk8s status --wait-ready
- sudo echo "Joining"
- sudo echo "Will join {{.IPOfNodeToJoin}}:{{.PortOfNodeToJoin}}"
- sudo sh -c "while ! microk8s join {{.IPOfNodeToJoin}}:{{.PortOfNodeToJoin}}/{{.JoinToken}} --worker ; do sleep 10 ; echo 'Retry join'; done"
`
)

// WorkerJoinInput defines context to generate instance user data for worker nodes to join.
type WorkerJoinInput struct {
	BaseUserData
	secret.Certificates
	JoinToken        string
	IPOfNodeToJoin   string
	PortOfNodeToJoin string
	Version          string
}

// NewJoinWorker returns the user data string to be used on a new worker instance.
func NewJoinWorker(input *WorkerJoinInput) ([]byte, error) {
	input.WriteFiles = input.Certificates.AsFiles()
	input.ControlPlane = false
	if err := input.prepare(); err != nil {
		return nil, err
	}
	major, minor, err := extractVersionParts(input.Version)
	if err != nil {
		return nil, err
	}
	input.Version = generateSnapChannelArgument(major, minor)

	userData, err := generate("JoinWorker", workerJoinCloudInit, input)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate user data for machine joining as worker")
	}

	return userData, err
}
