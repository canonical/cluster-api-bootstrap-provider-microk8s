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
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/cluster-api-bootstrap-provider-microk8s/apis/v1beta1"
	"github.com/canonical/cluster-api-bootstrap-provider-microk8s/controllers/cloudinit"
	. "github.com/onsi/gomega"
)

func TestCloudConfigInput(t *testing.T) {
	t.Run("ChannelAndRiskLevel", func(t *testing.T) {
		for _, tc := range []struct {
			name            string
			makeCloudConfig func(confinement, risk string) (*cloudinit.CloudConfig, error)
		}{
			{
				name: "ControlPlaneInit",
				makeCloudConfig: func(confinement, risk string) (*cloudinit.CloudConfig, error) {
					return cloudinit.NewInitControlPlane(&cloudinit.ControlPlaneInitInput{
						KubernetesVersion: "v1.25.0",
						Confinement:       confinement,
						Token:             strings.Repeat("a", 32),
						TokenTTL:          100,
						RiskLevel:         risk,
					})
				},
			},
			{
				name: "ControlPlaneJoin",
				makeCloudConfig: func(confinement, risk string) (*cloudinit.CloudConfig, error) {
					return cloudinit.NewJoinControlPlane(&cloudinit.ControlPlaneJoinInput{
						KubernetesVersion: "v1.25.0",
						Confinement:       confinement,
						Token:             strings.Repeat("a", 32),
						TokenTTL:          100,
						RiskLevel:         risk,
					})
				},
			},
			{
				name: "Worker",
				makeCloudConfig: func(confinement, risk string) (*cloudinit.CloudConfig, error) {
					return cloudinit.NewJoinWorker(&cloudinit.WorkerInput{
						KubernetesVersion: "v1.25.0",
						Confinement:       confinement,
						Token:             strings.Repeat("a", 32),
						RiskLevel:         risk,
					})
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				for _, confinement := range []string{"strict", "classic"} {
					t.Run(confinement, func(t *testing.T) {
						for _, risk := range []string{"stable", "candidate", "beta", "edge"} {
							t.Run(risk, func(t *testing.T) {
								g := NewWithT(t)
								c, err := tc.makeCloudConfig(confinement, risk)
								g.Expect(err).NotTo(HaveOccurred())

								if confinement == "classic" {
									g.Expect(c.RunCommands).To(ContainElement(fmt.Sprintf(`/capi-scripts/00-install-microk8s.sh "--channel 1.25/%s --classic"`, risk)))
								} else {
									g.Expect(c.RunCommands).To(ContainElement(fmt.Sprintf(`/capi-scripts/00-install-microk8s.sh "--channel 1.25-strict/%s"`, risk)))
								}

								_, err = cloudinit.GenerateCloudConfig(c)
								g.Expect(err).NotTo(HaveOccurred())
							})
						}
					})
				}
			})
		}
	})

	t.Run("ExtraWriteFiles", func(t *testing.T) {
		files := []v1beta1.CloudInitWriteFile{{
			Content:     "contents",
			Path:        "/tmp/path",
			Permissions: "0644",
			Owner:       "root:root",
		}}
		for _, tc := range []struct {
			name            string
			makeCloudConfig func() (*cloudinit.CloudConfig, error)
		}{
			{
				name: "ControlPlaneInit",
				makeCloudConfig: func() (*cloudinit.CloudConfig, error) {
					return cloudinit.NewInitControlPlane(&cloudinit.ControlPlaneInitInput{
						KubernetesVersion: "v1.25.0",
						Token:             strings.Repeat("a", 32),
						TokenTTL:          100,
						ExtraWriteFiles:   cloudinit.WriteFilesFromAPI(files),
					})
				},
			},
			{
				name: "ControlPlaneJoin",
				makeCloudConfig: func() (*cloudinit.CloudConfig, error) {
					return cloudinit.NewJoinControlPlane(&cloudinit.ControlPlaneJoinInput{
						KubernetesVersion: "v1.25.0",
						Token:             strings.Repeat("a", 32),
						TokenTTL:          100,
						ExtraWriteFiles:   cloudinit.WriteFilesFromAPI(files),
					})
				},
			},
			{
				name: "Worker",
				makeCloudConfig: func() (*cloudinit.CloudConfig, error) {
					return cloudinit.NewJoinWorker(&cloudinit.WorkerInput{
						KubernetesVersion: "v1.25.0",
						Token:             strings.Repeat("a", 32),
						ExtraWriteFiles:   cloudinit.WriteFilesFromAPI(files),
					})
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				g := NewWithT(t)
				c, err := tc.makeCloudConfig()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(c.WriteFiles).To(ContainElement(cloudinit.File{
					Content:     "contents",
					Path:        "/tmp/path",
					Permissions: "0644",
					Owner:       "root:root",
				}))

				_, err = cloudinit.GenerateCloudConfig(c)
				g.Expect(err).NotTo(HaveOccurred())
			})
		}
	})

	t.Run("ExtraKubeletArgs", func(t *testing.T) {
		for _, tc := range []struct {
			name            string
			makeCloudConfig func(args []string) (*cloudinit.CloudConfig, error)
		}{
			{
				name: "ControlPlaneInit",
				makeCloudConfig: func(args []string) (*cloudinit.CloudConfig, error) {
					return cloudinit.NewInitControlPlane(&cloudinit.ControlPlaneInitInput{
						KubernetesVersion: "v1.25.0",
						Token:             strings.Repeat("a", 32),
						TokenTTL:          100,
						ExtraKubeletArgs:  args,
					})
				},
			},
			{
				name: "ControlPlaneJoin",
				makeCloudConfig: func(args []string) (*cloudinit.CloudConfig, error) {
					return cloudinit.NewJoinControlPlane(&cloudinit.ControlPlaneJoinInput{
						KubernetesVersion: "v1.25.0",
						Token:             strings.Repeat("a", 32),
						TokenTTL:          100,
						ExtraKubeletArgs:  args,
					})
				},
			},
			{
				name: "Worker",
				makeCloudConfig: func(args []string) (*cloudinit.CloudConfig, error) {
					return cloudinit.NewJoinWorker(&cloudinit.WorkerInput{
						KubernetesVersion: "v1.25.0",
						Token:             strings.Repeat("a", 32),
						ExtraKubeletArgs:  args,
					})
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				for _, withArgs := range []bool{true, false} {
					t.Run(fmt.Sprintf("withargs=%v", withArgs), func(t *testing.T) {
						g := NewWithT(t)
						var args []string
						if withArgs {
							args = []string{"--arg=value", "--arg2=value2"}
						}
						c, err := tc.makeCloudConfig(args)
						g.Expect(err).NotTo(HaveOccurred())

						_, err = cloudinit.GenerateCloudConfig(c)
						g.Expect(err).NotTo(HaveOccurred())
						if withArgs {
							g.Expect(c.WriteFiles).To(ContainElement(cloudinit.File{
								Content:     "--arg=value\n--arg2=value2",
								Path:        "/var/tmp/extra-kubelet-args",
								Permissions: "0400",
								Owner:       "root:root",
							}))
						} else {
							for _, f := range c.WriteFiles {
								g.Expect(f.Path).ToNot(Equal("/var/tmp/extra-kubelet-args"))
							}
						}
					})
				}

			})
		}
	})
}
