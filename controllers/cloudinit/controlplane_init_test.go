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

func TestControlPlaneInit(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		g := NewWithT(t)

		cloudConfig, err := cloudinit.NewInitControlPlane(&cloudinit.ControlPlaneInitInput{
			CAKey:                `CA KEY DATA`,
			CACert:               `CA CERT DATA`,
			ControlPlaneEndpoint: "k8s.my-domain.com",
			KubernetesVersion:    "v1.25.2",
			ClusterAgentPort:     "30000",
			DqlitePort:           "2379",
			IPinIP:               true,
			Token:                strings.Repeat("a", 32),
			TokenTTL:             10000,
		})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(cloudConfig.RunCommands).To(Equal([]string{
			`set -x`,
			`/capi-scripts/00-disable-host-services.sh`,
			`/capi-scripts/00-install-microk8s.sh "1.25 --classic"`,
			`/capi-scripts/10-configure-containerd-proxy.sh "" "" ""`,
			`microk8s status --wait-ready`,
			`microk8s refresh-certs /var/tmp`,
			`/capi-scripts/10-configure-calico-ipip.sh true`,
			`/capi-scripts/10-configure-cluster-agent-port.sh "30000"`,
			`/capi-scripts/10-configure-dqlite-port.sh "2379"`,
			`/capi-scripts/10-configure-apiserver.sh "DNS" "k8s.my-domain.com"`,
			`/capi-scripts/20-microk8s-enable.sh "dns"`,
			`microk8s add-node --token-ttl 10000 --token "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
		}))

		g.Expect(cloudConfig.WriteFiles).To(ContainElements(
			cloudinit.File{
				Content:     "CA KEY DATA",
				Path:        "/var/tmp/ca.key",
				Permissions: "0600",
				Owner:       "root:root",
			},
			cloudinit.File{
				Content:     "CA CERT DATA",
				Path:        "/var/tmp/ca.crt",
				Permissions: "0600",
				Owner:       "root:root",
			},
		))

		_, err = cloudinit.GenerateCloudConfig(cloudConfig)
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestControlPlaneConfinementInit(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		g := NewWithT(t)

		cloudConfig, err := cloudinit.NewInitControlPlane(&cloudinit.ControlPlaneInitInput{
			CAKey:                `CA KEY DATA`,
			CACert:               `CA CERT DATA`,
			ControlPlaneEndpoint: "k8s.my-domain.com",
			KubernetesVersion:    "v1.25.2",
			ClusterAgentPort:     "30000",
			DqlitePort:           "2379",
			IPinIP:               true,
			Token:                strings.Repeat("a", 32),
			TokenTTL:             10000,
			Confinement:          "strict",
		})
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(cloudConfig.RunCommands).To(Equal([]string{
			`set -x`,
			`/capi-scripts/00-disable-host-services.sh`,
			`/capi-scripts/00-install-microk8s.sh "1.25-strict"`,
			`/capi-scripts/10-configure-containerd-proxy.sh "" "" ""`,
			`microk8s status --wait-ready`,
			`microk8s refresh-certs /var/tmp`,
			`/capi-scripts/10-configure-calico-ipip.sh true`,
			`/capi-scripts/10-configure-cluster-agent-port.sh "30000"`,
			`/capi-scripts/10-configure-dqlite-port.sh "2379"`,
			`/capi-scripts/10-configure-apiserver.sh "DNS" "k8s.my-domain.com"`,
			`/capi-scripts/20-microk8s-enable.sh "dns"`,
			`microk8s add-node --token-ttl 10000 --token "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
		}))

		g.Expect(cloudConfig.WriteFiles).To(ContainElements(
			cloudinit.File{
				Content:     "CA KEY DATA",
				Path:        "/var/tmp/ca.key",
				Permissions: "0600",
				Owner:       "root:root",
			},
			cloudinit.File{
				Content:     "CA CERT DATA",
				Path:        "/var/tmp/ca.crt",
				Permissions: "0600",
				Owner:       "root:root",
			},
		))

		_, err = cloudinit.GenerateCloudConfig(cloudConfig)
		g.Expect(err).ToNot(HaveOccurred())
	})
}
