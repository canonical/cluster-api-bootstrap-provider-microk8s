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

		joins := [2]string{"10.0.3.194", "10.0.3.195"}
		cloudConfig, err := cloudinit.NewJoinWorker(&cloudinit.WorkerInput{
			ControlPlaneEndpoint: "capi-aws-apiserver-1647391446.us-east-1.elb.amazonaws.com",
			KubernetesVersion:    "v1.24.3",
			ClusterAgentPort:     "30000",
			Token:                strings.Repeat("a", 32),
			JoinNodeIPs:          joins,
		})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(cloudConfig.RunCommands).To(Equal([]string{
			`set -x`,
			`/capi-scripts/00-configure-snapstore-http-proxy.sh "" ""`,
			`/capi-scripts/00-configure-snapstore-proxy.sh "" ""`,
			`/capi-scripts/00-disable-host-services.sh`,
			`/capi-scripts/00-install-microk8s.sh "--channel 1.24 --classic"`,
			`/capi-scripts/10-configure-containerd-proxy.sh "" "" ""`,
			`/capi-scripts/10-configure-kubelet.sh`,
			`microk8s status --wait-ready`,
			`/capi-scripts/10-configure-cluster-agent-port.sh "30000"`,
			`/capi-scripts/20-microk8s-join.sh yes "10.0.3.194:30000/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" "10.0.3.195:30000/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
			`/capi-scripts/30-configure-traefik.sh capi-aws-apiserver-1647391446.us-east-1.elb.amazonaws.com 6443 no`,
		}))

		_, err = cloudinit.GenerateCloudConfig(cloudConfig)
		g.Expect(err).ToNot(HaveOccurred())
	})
}
