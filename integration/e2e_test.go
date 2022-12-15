//go:build integration
// +build integration

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

package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

const KUBECONFIG string = "/var/tmp/kubeconfig.e2e.conf"

func init() {
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-s
		teardownCluster()
		os.Exit(1)
	}()
}

// TestBasic waits for the target cluster to deploy and start a 30 pod deployment.
// The CLUSTER_MANIFEST_FILE environment variable should point to a manifest with the target cluster
// kubectl and clusterctl have to be avaibale in the caller's path.
// kubectl should be setup so it uses the kubeconfig of the management cluster by default.
func TestBasic(t *testing.T) {
	setupCheck(t)
	t.Cleanup(teardownCluster)

	t.Run("DeployCluster", func(t *testing.T) { deployCluster(t) })
	t.Run("DeployMicrobot", func(t *testing.T) { deployMicrobot(t) })
	t.Run("UpgradeCluster", func(t *testing.T) { upgradeCluster(t) })
}

func setupCheck(t testing.TB) {
	cluster_manifest_file := os.Getenv("CLUSTER_MANIFEST_FILE")
	if cluster_manifest_file == "" {
		t.Fatalf("Environment variable CLUSTER_MANIFEST_FILE is not set. " +
			"CLUSTER_MANIFEST_FILE is expected to hold the PATH to a cluster manifest.")
	}
	t.Logf("Cluster to setup is in %s", cluster_manifest_file)

	_, err := exec.LookPath("kubectl")
	if err != nil {
		t.Fatalf("Please make sure kubectl is in your PATH. %s", err)
	}

	_, err = exec.LookPath("clusterctl")
	if err != nil {
		t.Fatalf("Please make sure clusterctl is in your PATH. %s", err)
	}

	t.Logf("Waiting for the MicroK8s providers to deploy on the management cluster.")
	waitForPod(t, "capi-microk8s-bootstrap-controller-manager", "capi-microk8s-bootstrap-system")
	waitForPod(t, "capi-microk8s-control-plane-controller-manager", "capi-microk8s-control-plane-system")
}

func waitForPod(t testing.TB, pod string, ns string) {
	attempt := 0
	maxAttempts := 10
	command := []string{"kubectl", "wait", "--timeout=15s", "--for=condition=available", "deploy/" + pod, "-n", ns}
	for {
		cmd := exec.Command(command[0], command[1:]...)
		outputBytes, err := cmd.CombinedOutput()
		if err != nil {
			t.Log(string(outputBytes))
			if attempt >= maxAttempts {
				t.Fatal(err)
			} else {
				t.Logf("Retrying...")
				attempt++
				time.Sleep(10 * time.Second)
			}
		} else {
			break
		}
	}
}

func teardownCluster() {
	command := []string{"kubectl", "get", "cluster", "--no-headers", "-o", "custom-columns=:metadata.name"}
	cmd := exec.Command(command[0], command[1:]...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		// Could not get any produced clusters. Nothing to cleanup... I hope.
		return
	}

	cluster := strings.Trim(string(outputBytes), "\n")
	command = []string{"kubectl", "delete", "cluster", cluster}
	cmd = exec.Command(command[0], command[1:]...)
	cmd.Run()
}

func deployCluster(t testing.TB) {
	t.Log("Setting up the cluster")
	command := []string{"kubectl", "apply", "-f", os.Getenv("CLUSTER_MANIFEST_FILE")}
	cmd := exec.Command(command[0], command[1:]...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(string(outputBytes))
		t.Fatalf("Failed to create the requested cluster. %s", err)
	}

	time.Sleep(30 * time.Second)

	command = []string{"kubectl", "get", "cluster", "--no-headers", "-o", "custom-columns=:metadata.name"}
	cmd = exec.Command(command[0], command[1:]...)
	outputBytes, err = cmd.CombinedOutput()
	if err != nil {
		t.Error(string(outputBytes))
		t.Fatalf("Failed to get the name of the cluster. %s", err)
	}

	cluster := strings.Trim(string(outputBytes), "\n")
	t.Logf("Cluster name is %s", cluster)

	attempt := 0
	maxAttempts := 60
	command = []string{"clusterctl", "get", "kubeconfig", cluster}
	for {
		cmd = exec.Command(command[0], command[1:]...)
		outputBytes, err = cmd.Output()
		if err != nil {
			if attempt >= maxAttempts {
				t.Fatal(err)
			} else {
				attempt++
				t.Log("Failed to get the target's kubeconfig, retrying.")
				time.Sleep(20 * time.Second)
			}
		} else {
			cfg := strings.Trim(string(outputBytes), "\n")
			err = os.WriteFile(KUBECONFIG, []byte(cfg), 0644)
			if err != nil {
				t.Fatalf("Could not persist the targets kubeconfig file. %s", err)
			}
			t.Logf("Target's kubeconfig file is at %s", KUBECONFIG)
			t.Log(cfg)
			break
		}
	}

	// Wait until the cluster is provisioned
	attempt = 0
	maxAttempts = 60
	command = []string{"kubectl", "get", "cluster", cluster}
	for {
		cmd = exec.Command(command[0], command[1:]...)
		outputBytes, err = cmd.CombinedOutput()
		if err != nil {
			t.Log(string(outputBytes))
			if attempt >= maxAttempts {
				t.Fatal(err)
			} else {
				attempt++
				t.Log("Retrying")
				time.Sleep(10 * time.Second)
			}
		} else {
			if strings.Contains(string(outputBytes), "Provisioned") {
				break
			} else {
				attempt++
				time.Sleep(20 * time.Second)
				t.Log("Waiting for the cluster to be provisioned")
			}
		}
	}

	// Wait until all machines are running
	attempt = 0
	maxAttempts = 60
	machines := 0
	command = []string{"kubectl", "get", "machine", "--no-headers"}
	for {
		cmd = exec.Command(command[0], command[1:]...)
		outputBytes, err = cmd.CombinedOutput()
		output := string(outputBytes)
		if err != nil {
			t.Log(output)
			if attempt >= maxAttempts {
				t.Fatal(err)
			} else {
				attempt++
				t.Log("Retrying")
				time.Sleep(10 * time.Second)
			}
		} else {
			machines = strings.Count(output, "\n")
			running := strings.Count(output, "Running")
			t.Logf("Machines %d out of which %d are Running", machines, running)
			if machines == running {
				break
			} else {
				attempt++
				time.Sleep(10 * time.Second)
				t.Log("Waiting for machines to start running")
			}
		}
	}

	// Make sure we have as many nodes as machines
	attempt = 0
	maxAttempts = 60
	command = []string{"kubectl", "--kubeconfig=" + KUBECONFIG, "get", "no", "--no-headers"}
	for {
		cmd = exec.Command(command[0], command[1:]...)
		outputBytes, err = cmd.CombinedOutput()
		output := string(outputBytes)
		if err != nil {
			t.Log(output)
			if attempt >= maxAttempts {
				t.Fatal(err)
			} else {
				attempt++
				time.Sleep(10 * time.Second)
				t.Log("Retrying")
			}
		} else {
			nodes := strings.Count(output, "\n")
			ready := strings.Count(output, " Ready")
			t.Logf("Machines are %d, Nodes are %d out of which %d are Ready", machines, nodes, ready)
			if machines == nodes && ready == nodes {
				break
			} else {
				attempt++
				time.Sleep(20 * time.Second)
				t.Log("Waiting for nodes to become ready")
			}
		}
	}
}

func deployMicrobot(t testing.TB) {
	t.Log("Deploying microbot")

	command := []string{"kubectl", "--kubeconfig=" + KUBECONFIG, "create", "deploy", "--image=cdkbot/microbot:1", "--replicas=30", "bot"}
	cmd := exec.Command(command[0], command[1:]...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(string(outputBytes))
		t.Fatalf("Failed to create the requested microbot deployment. %s", err)
	}

	// Make sure we have as many nodes as machines
	attempt := 0
	maxAttempts := 60
	t.Log("Waiting for the deployment to complete")
	command = []string{"kubectl", "--kubeconfig=" + KUBECONFIG, "wait", "deploy/bot", "--for=jsonpath={.status.readyReplicas}=30"}
	for {
		cmd = exec.Command(command[0], command[1:]...)
		outputBytes, err := cmd.CombinedOutput()
		if err != nil {
			t.Log(string(outputBytes))
			if attempt >= maxAttempts {
				t.Fatal(err)
			} else {
				attempt++
				t.Log("Retrying")
				time.Sleep(10 * time.Second)
			}
		} else {
			break
		}
	}
}

func upgradeCluster(t testing.TB) {
	version := os.Getenv("CAPI_UPGRADE_VERSION")
	if version == "" {
		t.Fatalf("Environment variable CAPI_UPGRADE_VERSION is not set." +
			"Please set it to the version you want to upgrade to.")
	}

	control_plane_name := os.Getenv("CAPI_UPGRADE_CP_NAME")
	if control_plane_name == "" {
		t.Fatalf("Environment variable CAPI_UPGRADE_CP_NAME is not set." +
			"Please set it to the name of the control plane you want to upgrade.")
	}

	control_plane_type := os.Getenv("CAPI_UPGRADE_CP_TYPE")
	if control_plane_type == "" {
		t.Fatalf("Environment variable CAPI_UPGRADE_CP_TYPE is not set." +
			"Please set it to the type of the control plane you want to upgrade.")
	}

	worker_deployment_name := os.Getenv("CAPI_UPGRADE_MD_NAME")
	if worker_deployment_name == "" {
		t.Fatalf("Environment variable CAPI_UPGRADE_MD_NAME is not set." +
			"Please set it to the name of the machine deployment you want to upgrade.")
	}

	worker_deployment_type := os.Getenv("CAPI_UPGRADE_MD_TYPE")
	if worker_deployment_type == "" {
		t.Fatalf("Environment variable CAPI_UPGRADE_MD_TYPE is not set." +
			"Please set it to the type of the machine deployment you want to upgrade.")
	}

	t.Logf("Upgrading cluster to %s", version)
	// Patch contol plane machine upgrades.
	command := []string{"kubectl", "patch", "--type=merge", control_plane_type, control_plane_name, "--patch",
		fmt.Sprintf(`{"spec":{"version":"%s"}}`, version)}
	cmd := exec.Command(command[0], command[1:]...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(string(outputBytes))
		t.Fatalf("Failed to merge the patch to control plane. %s", err)
	}

	// Patch worker machine upgrades.
	command = []string{"kubectl", "patch", "--type=merge", worker_deployment_type, worker_deployment_name, "--patch",
		fmt.Sprintf(`{"spec":{"template":{"spec":{"version":"%s"}}}}`, version)}
	cmd = exec.Command(command[0], command[1:]...)
	outputBytes, err = cmd.CombinedOutput()
	if err != nil {
		t.Error(string(outputBytes))
		t.Fatalf("Failed to merge the patch to the machine deployments. %s", err)
	}

	time.Sleep(30 * time.Second)

	// Now all the machines should be upgraded to the new version.
	attempt := 0
	maxAttempts := 60
	command = []string{"kubectl", "get", "machine", "--no-headers"}
	for {
		cmd := exec.Command(command[0], command[1:]...)
		outputBytes, err := cmd.CombinedOutput()
		output := string(outputBytes)
		if err != nil {
			t.Log(output)
			if attempt >= maxAttempts {
				t.Fatal(err)
			}

			attempt++
			t.Log("Retrying")
			time.Sleep(20 * time.Second)
		} else {
			totalMachines := strings.Count(output, "Running")

			// We count all the "Running" machines with the new version.
			re := regexp.MustCompile("Running .* " + version)
			upgradedMachines := len(re.FindAllString(output, -1))
			t.Logf("Total machines %d out of which %d are upgraded", totalMachines, upgradedMachines)
			if totalMachines == upgradedMachines {
				break
			} else {
				attempt++
				time.Sleep(20 * time.Second)
				t.Log("Waiting for machines to upgrade and start running")
			}
		}
	}
}
