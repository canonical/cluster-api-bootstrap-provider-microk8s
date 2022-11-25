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
	"fmt"

	"k8s.io/apimachinery/pkg/util/version"
)

func createInstallArgs(confinement string, riskLevel string, kubernetesVersion *version.Version) string {
	installChannel := fmt.Sprintf("%d.%d", kubernetesVersion.Major(), kubernetesVersion.Minor())
	var installArgs string
	if confinement == "strict" {
		if riskLevel != "" {
			installArgs = fmt.Sprintf("--channel %s-strict/%s", installChannel, riskLevel)
		} else {
			installArgs = fmt.Sprintf("--channel %s-strict", installChannel)
		}
	} else {
		if riskLevel != "" {
			installArgs = fmt.Sprintf("--channel %s/%s --classic", installChannel, riskLevel)
		} else {
			installArgs = fmt.Sprintf("--channel %s --classic", installChannel)
		}
	}

	return installArgs
}
