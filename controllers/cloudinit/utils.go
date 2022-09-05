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
	"bytes"
	"fmt"
	"strings"
	"text/template"

	version "github.com/hashicorp/go-version"
)

var (
	defaultTemplateFuncMap = template.FuncMap{
		"Indent": templateYAMLIndent,
	}
)

func templateYAMLIndent(i int, input string) string {
	split := strings.Split(input, "\n")
	ident := "\n" + strings.Repeat(" ", i)
	return strings.Repeat(" ", i) + strings.Join(split, ident)
}

func extractVersionParts(versionStr string) (int, int, error) {
	v, err := version.NewVersion(versionStr)
	if err != nil {
		return 0, 0, err
	}
	segs := v.Segments()
	return segs[0], segs[1], nil
}

func generateSnapChannelArgument(major int, minor int) string {
	return fmt.Sprintf("--channel=%d.%d", major, minor)
}

func generateProxyCommands(https *string, http *string, noproxy *string) string {
	var proxyCommands bytes.Buffer
	if https != nil {
		var cmd = fmt.Sprintf(
			"- sudo sh -c \"echo HTTPS_PROXY=%s >> /var/snap/microk8s/current/args/containerd-env\"",
			*https)
		proxyCommands.WriteString(cmd)
	}
	if http != nil {
		var cmd = fmt.Sprintf(
			"- sudo sh -c \"echo HTTP_PROXY=%s >> /var/snap/microk8s/current/args/containerd-env\"",
			*http)
		proxyCommands.WriteString(cmd)
	}
	if noproxy != nil {
		var cmd = fmt.Sprintf(
			"- sudo sh -c \"echo NO_PROXY=%s >> /var/snap/microk8s/current/args/containerd-env\"",
			*noproxy)
		proxyCommands.WriteString(cmd)
	}
	if proxyCommands.Len() == 0 {
		proxyCommands.WriteString("- sudo echo \"No proxy settings specified\"")
	}

	return proxyCommands.String()
}
