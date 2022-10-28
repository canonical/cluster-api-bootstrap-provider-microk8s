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
	"bytes"
	"testing"
	"text/template"

	. "github.com/onsi/gomega"
)

func TestTemplate(t *testing.T) {
	t.Run("ToYAML", func(t *testing.T) {
		g := NewWithT(t)
		tmpl, err := template.New("TestTemplate").Funcs(templateFuncsMap).Parse(`{{ . | ToYAML }}`)
		g.Expect(err).NotTo(HaveOccurred())

		b := &bytes.Buffer{}

		t.Run("Valid", func(t *testing.T) {
			g := NewWithT(t)
			err = tmpl.Execute(b, map[string]any{"key": "value"})

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(b.String()).To(Equal("key: value\n"))
		})

		t.Run("Invalid", func(t *testing.T) {
			g := NewWithT(t)

			err = tmpl.Execute(b, func() {})
			g.Expect(err).To(HaveOccurred())
		})
	})
}
