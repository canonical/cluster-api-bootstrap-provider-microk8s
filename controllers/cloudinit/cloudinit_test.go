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
	"testing"

	"github.com/canonical/cluster-api-bootstrap-provider-microk8s/controllers/cloudinit"
	. "github.com/onsi/gomega"
)

var (
	expectedCloudConfig string = `## template: jinja
#cloud-config
write_files:
- content: test file
  path: /run/a.tmp
  permissions: "0600"
  owner: root:root
runcmd:
- foo
- 'bar: test'
bootcmd:
- baz
`
)

func TestCloudConfig(t *testing.T) {
	g := NewWithT(t)
	cloudConfig := &cloudinit.CloudConfig{
		WriteFiles: []cloudinit.File{
			{
				Content:     "test file",
				Path:        "/run/a.tmp",
				Owner:       "root:root",
				Permissions: "0600",
			},
		},
		RunCommands: []string{
			"foo",
			"bar: test",
		},
		BootCommands: []string{
			"baz",
		},
	}

	b, err := cloudinit.GenerateCloudConfig(cloudConfig)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(b)).To(Equal(expectedCloudConfig))
}
