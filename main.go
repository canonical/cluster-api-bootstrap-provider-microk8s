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

package main

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/canonical/cluster-api-bootstrap-provider-microk8s/controllers"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2/klogr"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	bootstrapclusterxk8siov1beta1 "github.com/canonical/cluster-api-bootstrap-provider-microk8s/apis/v1beta1"

	//+kubebuilder:scaffold:imports
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = expv1.AddToScheme(scheme)
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(bootstrapclusterxk8siov1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	rand.Seed(time.Now().UnixNano())
}

func main() {
	// Set the Klog format, as the Serialize format shouldn't be used anymore.
	// This makes sure that the logs are formatted correctly, i.e.:
	// * JSON logging format: msg isn't serialized twice
	// * text logging format: values are formatted with their .String() func.
	ctrl.SetLogger(klogr.NewWithOptions(klogr.WithFormat(klogr.FormatKlog)))

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	restConfig := ctrl.GetConfigOrDie()
	restConfig.UserAgent = remote.DefaultClusterAPIUserAgent("cluster-api-microk8s-bootstrap-manager")
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "microk8s-bootstrap-manager-leader-election-capi",
		ClientDisableCacheFor: []client.Object{
			&corev1.ConfigMap{},
			&corev1.Secret{},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.MicroK8sConfigReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(context.TODO(),
		mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MicroK8sConfig")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
