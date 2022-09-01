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
	"testing"

	. "github.com/onsi/gomega"
)

func TestNewInitControlPlaneCommands(t *testing.T) {
	g := NewWithT(t)

	addons := []string{"foo", "bar"}
	cpinput := &ControlPlaneInput{
		ControlPlaneEndpoint: "lb-ep",
		JoinToken:            "my_join_token",
		JoinTokenTTLInSecs:   56789,
		Version:              "v1.23.3",
		Addons:               addons,
	}

	out, err := NewInitControlPlane(cpinput)
	g.Expect(err).NotTo(HaveOccurred())

	expectedCommands := []string{
		`snap install microk8s --classic --channel=1.23`,
		`microk8s add-node --token-ttl 56789 --token my_join_token`,
		`for a in  'foo'  'bar'  'dns'`,
		`sudo echo "No proxy settings specified"`,
	}

	a := string(out)
	for _, f := range expectedCommands {
		g.Expect(a).To(ContainSubstring(f))
	}

	http := "http://proxy"
	cpinputproxy := &ControlPlaneInput{
		Version:   "v1.23.3",
		HttpProxy: &http,
	}

	out, err = NewInitControlPlane(cpinputproxy)
	g.Expect(err).NotTo(HaveOccurred())

	expectedCommands = []string{
		`HTTP_PROXY=http://proxy`,
	}

	a = string(out)
	for _, f := range expectedCommands {
		g.Expect(a).To(ContainSubstring(f))
	}

}

func TestNewJoinControlPlaneCommands(t *testing.T) {
	g := NewWithT(t)

	cpinput := &ControlPlaneJoinInput{
		ControlPlaneEndpoint: "lb-ep",
		JoinToken:            "my_join_token",
		JoinTokenTTLInSecs:   56789,
		Version:              "v1.24.3",
		IPOfNodeToJoin:       "1.2.3.4",
		PortOfNodeToJoin:     "25000",
	}

	out, err := NewJoinControlPlane(cpinput)
	g.Expect(err).NotTo(HaveOccurred())

	expectedCommands := []string{
		`snap install microk8s --classic --channel=1.24`,
		`microk8s add-node --token-ttl 56789 --token my_join_token`,
		`microk8s join 1.2.3.4:25000`,
	}

	for _, f := range expectedCommands {
		g.Expect(string(out)).To(ContainSubstring(f))
	}
}

func TestNewJoinWorkerCommands(t *testing.T) {
	g := NewWithT(t)

	cpinput := &WorkerJoinInput{
		JoinToken:        "my_join_token",
		Version:          "v1.27.3",
		IPOfNodeToJoin:   "5.4.3.2",
		PortOfNodeToJoin: "23000",
	}

	out, err := NewJoinWorker(cpinput)
	g.Expect(err).NotTo(HaveOccurred())

	expectedCommands := []string{
		`snap install microk8s --classic --channel=1.27`,
		`microk8s join 5.4.3.2:23000/my_join_token --worker`,
	}

	for _, f := range expectedCommands {
		g.Expect(string(out)).To(ContainSubstring(f))
	}
}
