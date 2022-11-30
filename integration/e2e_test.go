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

	deployCluster(t)
	deployMicrobot(t)
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
	max_attempts := 10
	command_str := fmt.Sprintf("kubectl wait --timeout=15s --for=condition=available deploy/%s -n %s", pod, ns)
	command := strings.Fields(command_str)
	for {
		cmd := exec.Command(command[0], command[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Log(string(output))
			if attempt >= max_attempts {
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
	command := strings.Fields("kubectl get cluster --no-headers -o custom-columns=:metadata.name")
	cmd := exec.Command(command[0], command[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Could not get any produced clusters. Nothing to cleanup... I hope.
		return
	}

	cluster := strings.Trim(string(output), "\n")
	command_str := fmt.Sprintf("kubectl delete cluster %s", cluster)
	command = strings.Fields(command_str)
	cmd = exec.Command(command[0], command[1:]...)
	cmd.Run()
}

func deployCluster(t testing.TB) {
	t.Log("Setting up the cluster")
	command_str := fmt.Sprintf("kubectl apply -f %s", os.Getenv("CLUSTER_MANIFEST_FILE"))
	command := strings.Fields(command_str)
	cmd := exec.Command(command[0], command[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(string(output))
		t.Fatalf("Failed to create the requested cluster. %s", err)
	}

	time.Sleep(30 * time.Second)

	command = strings.Fields("kubectl get cluster --no-headers -o custom-columns=:metadata.name")
	cmd = exec.Command(command[0], command[1:]...)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Error(string(output))
		t.Fatalf("Failed to get the name of the cluster. %s", err)
	}

	cluster := strings.Trim(string(output), "\n")
	t.Logf("Cluster name is %s", cluster)

	attempt := 0
	max_attempts := 60
	command_str = fmt.Sprintf("clusterctl get kubeconfig %s", cluster)
	command = strings.Fields(command_str)
	for {
		cmd = exec.Command(command[0], command[1:]...)
		output, err = cmd.Output()
		if err != nil {
			if attempt >= max_attempts {
				t.Fatal(err)
			} else {
				attempt++
				t.Log("Failed to get the target's kubeconfig, retrying.")
				time.Sleep(20 * time.Second)
			}
		} else {
			cfg := strings.Trim(string(output), "\n")
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
	max_attempts = 60
	command_str = fmt.Sprintf("kubectl get cluster %s", cluster)
	command = strings.Fields(command_str)
	for {
		cmd = exec.Command(command[0], command[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Log(string(output))
			if attempt >= max_attempts {
				t.Fatal(err)
			} else {
				attempt++
				t.Log("Retrying")
				time.Sleep(10 * time.Second)
			}
		} else {
			if strings.Contains(string(output), "Provisioned") {
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
	max_attempts = 60
	machines := 0
	command = strings.Fields("kubectl get machine --no-headers")
	for {
		cmd = exec.Command(command[0], command[1:]...)
		output, err = cmd.CombinedOutput()
		output_str := string(output)
		if err != nil {
			t.Log(output_str)
			if attempt >= max_attempts {
				t.Fatal(err)
			} else {
				attempt++
				t.Log("Retrying")
				time.Sleep(10 * time.Second)
			}
		} else {
			machines = strings.Count(output_str, "\n")
			running := strings.Count(output_str, "Running")
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
	max_attempts = 60
	command_str = fmt.Sprintf("kubectl --kubeconfig=%s get no --no-headers", KUBECONFIG)
	command = strings.Fields(command_str)
	for {
		cmd = exec.Command(command[0], command[1:]...)
		output, err = cmd.CombinedOutput()
		output_str := string(output)
		if err != nil {
			t.Log(output_str)
			if attempt >= max_attempts {
				t.Fatal(err)
			} else {
				attempt++
				time.Sleep(10 * time.Second)
				t.Log("Retrying")
			}
		} else {
			nodes := strings.Count(output_str, "\n")
			ready := strings.Count(output_str, " Ready")
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
	command_str := fmt.Sprintf("kubectl --kubeconfig=%s delete deployment bot", KUBECONFIG)
	command := strings.Fields(command_str)
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Run()

	command_str = fmt.Sprintf("kubectl --kubeconfig=%s create deploy --image=cdkbot/microbot:1 --replicas=30 bot", KUBECONFIG)
	command = strings.Fields(command_str)
	cmd = exec.Command(command[0], command[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(string(output))
		t.Fatalf("Failed to create the requested microbot deployment. %s", err)
	}

	// Make sure we have as many nodes as machines
	attempt := 0
	max_attempts := 60
	t.Log("Waiting for the deployment to complete")
	command_str = fmt.Sprintf("kubectl --kubeconfig=%s wait deploy/bot --for=jsonpath={.status.readyReplicas}=30", KUBECONFIG)
	command = strings.Fields(command_str)
	for {
		cmd = exec.Command(command[0], command[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Log(string(output))
			if attempt >= max_attempts {
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
