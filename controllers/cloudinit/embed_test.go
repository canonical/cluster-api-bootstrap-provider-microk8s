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
	"testing"

	. "github.com/onsi/gomega"
)

func TestScripts(t *testing.T) {
	for _, scriptName := range allScripts {
		t.Run(string(scriptName), func(t *testing.T) {
			g := NewWithT(t)
			s := mustGetScript(scriptName)

			g.Expect(s).NotTo(BeEmpty(), "script must not be empty")
		})
	}
}
