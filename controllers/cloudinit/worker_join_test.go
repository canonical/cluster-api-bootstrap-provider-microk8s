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

package cloudinit_test

import (
	"strings"
	"testing"

	"github.com/canonical/cluster-api-bootstrap-provider-microk8s/controllers/cloudinit"
	. "github.com/onsi/gomega"
)

func TestWorkerJoin(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		g := NewWithT(t)

		cloudConfig, err := cloudinit.NewJoinWorker(&cloudinit.WorkerInput{
			KubernetesVersion: "v1.24.3",
			ClusterAgentPort:  "30000",
			Token:             strings.Repeat("a", 32),
			JoinNodeIP:        "10.0.3.194",
		})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(cloudConfig.RunCommands).To(Equal([]string{
			`set -x`,
			`/capi-scripts/00-disable-host-services.sh`,
			`/capi-scripts/00-install-microk8s.sh "--channel 1.24 --classic"`,
			`/capi-scripts/10-configure-containerd-proxy.sh "" "" ""`,
			`microk8s status --wait-ready`,
			`/capi-scripts/10-configure-cluster-agent-port.sh "30000"`,
			`/capi-scripts/20-microk8s-join.sh "10.0.3.194:30000/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" --worker`,
		}))

		_, err = cloudinit.GenerateCloudConfig(cloudConfig)
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestConfinementWorkerJoin(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		g := NewWithT(t)

		cloudConfig, err := cloudinit.NewJoinWorker(&cloudinit.WorkerInput{
			KubernetesVersion: "v1.25.3",
			ClusterAgentPort:  "30000",
			Token:             strings.Repeat("a", 32),
			JoinNodeIP:        "10.0.3.194",
			Confinement:       "strict",
		})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(cloudConfig.RunCommands).To(Equal([]string{
			`set -x`,
			`/capi-scripts/00-disable-host-services.sh`,
			`/capi-scripts/00-install-microk8s.sh "--channel 1.25-strict"`,
			`/capi-scripts/10-configure-containerd-proxy.sh "" "" ""`,
			`microk8s status --wait-ready`,
			`/capi-scripts/10-configure-cluster-agent-port.sh "30000"`,
			`/capi-scripts/20-microk8s-join.sh "10.0.3.194:30000/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" --worker`,
		}))

		_, err = cloudinit.GenerateCloudConfig(cloudConfig)
		g.Expect(err).ToNot(HaveOccurred())
	})
}
