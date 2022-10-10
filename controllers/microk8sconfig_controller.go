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

package controllers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	mrand "math/rand"
	"time"

	bootstrapclusterxk8siov1beta1 "github.com/canonical/cluster-api-bootstrap-provider-microk8s/apis/v1beta1"
	cloudinit "github.com/canonical/cluster-api-bootstrap-provider-microk8s/controllers/cloudinit"

	"github.com/canonical/cluster-api-bootstrap-provider-microk8s/apis/v1beta1"

	"github.com/canonical/cluster-api-bootstrap-provider-microk8s/controllers/locking"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bsutil "sigs.k8s.io/cluster-api/bootstrap/util"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type InitLocker interface {
	Lock(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) bool
	Unlock(ctx context.Context, cluster *clusterv1.Cluster) bool
}

// MicroK8sConfigReconciler reconciles a MicroK8sConfig object
type MicroK8sConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
	MicroK8sInitLock InitLocker
}

// Scope is a scoped struct used during reconciliation.
type Scope struct {
	Config *bootstrapclusterxk8siov1beta1.MicroK8sConfig
	logr.Logger
	ConfigOwner *bsutil.ConfigOwner
	Cluster     *clusterv1.Cluster
}

const (
	defaultDqlitePort  string = "19001"
	remappedDqlitePort string = "2379"

	defaultClusterAgentPort  string = "25000"
	remappedClusterAgentPort string = "30000"
)

//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=microk8sconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=microk8sconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=microk8sconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/finalizers;clusters/status;machines;machines/finalizers;machines/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MicroK8sConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *MicroK8sConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	log := log.FromContext(ctx)

	config := &bootstrapclusterxk8siov1beta1.MicroK8sConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get config")
		return ctrl.Result{}, err
	}

	configOwner, err := bsutil.GetConfigOwner(ctx, r.Client, config)
	if apierrors.IsNotFound(err) {
		// Could not find the owner yet, this is not an error and will rereconcile when the owner gets set.
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, "Failed to get owner")
		return ctrl.Result{}, err
	}
	if configOwner == nil {
		return ctrl.Result{}, nil
	}
	log = log.WithValues("kind", configOwner.GetKind(), "version", configOwner.GetResourceVersion(), "name", configOwner.GetName())

	cluster, err := util.GetClusterByName(ctx, r.Client, configOwner.GetNamespace(), configOwner.ClusterName())
	if err != nil {
		if errors.Is(err, util.ErrNoCluster) {
			log.Info(fmt.Sprintf("%s does not belong to a cluster yet, waiting until it's part of a cluster", configOwner.GetKind()))
			return ctrl.Result{}, nil
		}

		if apierrors.IsNotFound(err) {
			log.Info("Cluster does not exist yet, waiting until it is created")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Could not get cluster with metadata")
		return ctrl.Result{}, err
	}

	if annotations.IsPaused(cluster, config) {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	scope := &Scope{
		Logger:      log,
		Config:      config,
		ConfigOwner: configOwner,
		Cluster:     cluster,
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(config, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Attempt to Patch the KubeadmConfig object and status after each reconciliation if no error occurs.
	defer func() {
		conditions.SetSummary(config,
			conditions.WithConditions(
				bootstrapclusterxk8siov1beta1.DataSecretAvailableCondition,
				bootstrapclusterxk8siov1beta1.CertificatesAvailableCondition,
			),
		)
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		if rerr == nil {
			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
		}
		if err := patchHelper.Patch(ctx, config, patchOpts...); err != nil {
			log.Error(rerr, "Failed to patch config")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	switch {
	// Wait for the infrastructure to be ready.
	case !cluster.Status.InfrastructureReady:
		log.Info("Cluster infrastructure is not ready, waiting")
		conditions.MarkFalse(config, bootstrapclusterxk8siov1beta1.DataSecretAvailableCondition, v1beta1.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	// Reconcile status for machines that already have a secret reference, but our status isn't up to date.
	// This case solves the pivoting scenario (or a backup restore) which doesn't preserve the status subresource on objects.
	case configOwner.DataSecretName() != nil && (!config.Status.Ready || config.Status.DataSecretName == nil):
		config.Status.Ready = true
		config.Status.DataSecretName = configOwner.DataSecretName()
		conditions.MarkTrue(config, bootstrapclusterxk8siov1beta1.DataSecretAvailableCondition)
		return ctrl.Result{}, nil
	// Status is ready means a config has been generated.
	case config.Status.Ready:
		// Just return as the config is already generated and need not be generated again.
		return ctrl.Result{}, nil
	}

	// Note: can't use IsFalse here because we need to handle the absence of the condition as well as false.
	if !conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
		log.Info("Cluster control plane is not initialized, waiting")
		return r.handleClusterNotInitialized(ctx, scope)
	}

	// Unlock any locks that might have been set during init process
	r.MicroK8sInitLock.Unlock(ctx, cluster)

	// it's a control plane join or a worker node?
	if configOwner.IsControlPlaneMachine() {
		log.Info("Reconciling control plane")
		return r.handleJoiningControlPlaneNode(ctx, scope)
	}

	log.Info("Reconciling worker")
	return r.handleJoiningWorkerNode(ctx, scope)
}

func (r *MicroK8sConfigReconciler) handleClusterNotInitialized(ctx context.Context, scope *Scope) (_ ctrl.Result, reterr error) {
	// initialize the DataSecretAvailableCondition if missing.
	// this is required in order to avoid the condition's LastTransitionTime to flicker in case of errors surfacing
	// using the DataSecretGeneratedFailedReason
	if conditions.GetReason(scope.Config, bootstrapclusterxk8siov1beta1.DataSecretAvailableCondition) != bootstrapclusterxk8siov1beta1.DataSecretGenerationFailedReason {
		conditions.MarkFalse(scope.Config, bootstrapclusterxk8siov1beta1.DataSecretAvailableCondition, clusterv1.WaitingForControlPlaneAvailableReason, clusterv1.ConditionSeverityInfo, "")
	}

	// if it's NOT a control plane machine, requeue
	if !scope.ConfigOwner.IsControlPlaneMachine() {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	machine := &clusterv1.Machine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(scope.ConfigOwner.Object, machine); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "cannot convert %s to Machine", scope.ConfigOwner.GetKind())
	}

	// acquire the init lock so that only the first machine configured
	// as control plane get processed here
	// if not the first, requeue
	if !r.MicroK8sInitLock.Lock(ctx, scope.Cluster, machine) {
		scope.Info("A control plane is already being initialized, requeueing until control plane is ready")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	defer func() {
		if reterr != nil {
			if !r.MicroK8sInitLock.Unlock(ctx, scope.Cluster) {
				reterr = kerrors.NewAggregate([]error{reterr, errors.New("failed to unlock the init lock")})
			}
		}
	}()

	scope.Info("Creating BootstrapData for the init control plane")

	microk8sConfig := &bootstrapclusterxk8siov1beta1.MicroK8sConfig{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: machine.Spec.Bootstrap.ConfigRef.Namespace,
		Name:      machine.Spec.Bootstrap.ConfigRef.Name,
	}, microk8sConfig); err != nil {
		return ctrl.Result{}, err
	}

	token, err := r.getJoinToken(ctx, scope)
	if err != nil {
		scope.Info("Failed to get or generate the join token, requeueing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	cert, key, err := r.getCA(ctx, scope)
	if err != nil {
		scope.Info("Failed to get or generate the CA, requeueing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	portOfClusterAgent := defaultClusterAgentPort
	portOfDqlite := defaultDqlitePort
	if microk8sConfig.Spec.ClusterConfiguration == nil ||
		microk8sConfig.Spec.ClusterConfiguration.PortCompatibilityRemap {
		portOfClusterAgent = remappedClusterAgentPort
		portOfDqlite = remappedDqlitePort
	}

	controlPlaneInput := &cloudinit.ControlPlaneInput{
		BaseUserData:         cloudinit.BaseUserData{},
		CACert:               *cert,
		CAKey:                *key,
		ControlPlaneEndpoint: scope.Cluster.Spec.ControlPlaneEndpoint.Host,
		JoinToken:            token,
		JoinTokenTTLInSecs:   microk8sConfig.Spec.InitConfiguration.JoinTokenTTLInSecs,
		Version:              *machine.Spec.Version,
		PortOfClusterAgent:   portOfClusterAgent,
		PortOfDqlite:         portOfDqlite,
	}
	if microk8sConfig.Spec.InitConfiguration != nil {
		if microk8sConfig.Spec.InitConfiguration.Addons != nil {
			controlPlaneInput.Addons = microk8sConfig.Spec.InitConfiguration.Addons
		}
		controlPlaneInput.HTTPSProxy = microk8sConfig.Spec.InitConfiguration.HTTPSProxy
		controlPlaneInput.HTTPProxy = microk8sConfig.Spec.InitConfiguration.HTTPProxy
		controlPlaneInput.NoProxy = microk8sConfig.Spec.InitConfiguration.NoProxy

		if microk8sConfig.Spec.InitConfiguration.IPinIP {
			controlPlaneInput.IPinIP = true
		}

		if microk8sConfig.Spec.InitConfiguration.JoinTokenTTLInSecs == 0 {
			// set by default to 10 years
			controlPlaneInput.JoinTokenTTLInSecs = 315569260
		}
	}

	bootstrapInitData, err := cloudinit.NewInitControlPlane(controlPlaneInput)

	if err != nil {
		scope.Error(err, "Failed to generate user data for bootstrap control plane")
		return ctrl.Result{}, err
	}

	if err := r.storeBootstrapData(ctx, scope, bootstrapInitData); err != nil {
		scope.Error(err, "Failed to store bootstrap data")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MicroK8sConfigReconciler) handleJoiningControlPlaneNode(ctx context.Context, scope *Scope) (_ ctrl.Result, reterr error) {
	// if it's NOT a control plane machine, requeue
	if !scope.ConfigOwner.IsControlPlaneMachine() {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	machine := &clusterv1.Machine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(scope.ConfigOwner.Object, machine); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "cannot convert %s to Machine", scope.ConfigOwner.GetKind())
	}

	// acquire the init lock so that only only one machine joins each time
	if !r.MicroK8sInitLock.Lock(ctx, scope.Cluster, machine) {
		scope.Info("A node is already being handled, requeueing until cluster can be extended with this node")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	defer func() {
		if reterr != nil {
			if !r.MicroK8sInitLock.Unlock(ctx, scope.Cluster) {
				reterr = kerrors.NewAggregate([]error{reterr, errors.New("failed to unlock the init lock")})
			}
		}
	}()

	scope.Info("Creating BootstrapData for the join control plane")
	ipOfNodeToConnectTo, err := r.getControlPlaneNodeToJoin(ctx, scope)
	if err != nil || ipOfNodeToConnectTo == "" {
		scope.Info("Failed to discover a control plane IP, requeueing.")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	microk8sConfig := &bootstrapclusterxk8siov1beta1.MicroK8sConfig{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: machine.Spec.Bootstrap.ConfigRef.Namespace,
		Name:      machine.Spec.Bootstrap.ConfigRef.Name,
	}, microk8sConfig); err != nil {
		return ctrl.Result{}, err
	}

	portOfNodeToConnectTo := defaultClusterAgentPort
	portOfDqlite := defaultDqlitePort
	if microk8sConfig.Spec.ClusterConfiguration == nil ||
		microk8sConfig.Spec.ClusterConfiguration.PortCompatibilityRemap {
		portOfNodeToConnectTo = remappedClusterAgentPort
		portOfDqlite = remappedDqlitePort
	}

	token, err := r.getJoinToken(ctx, scope)
	if err != nil {
		scope.Info("Failed to get or generate the join token, requeueing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	controlPlaneInput := &cloudinit.ControlPlaneJoinInput{
		BaseUserData:         cloudinit.BaseUserData{},
		ControlPlaneEndpoint: scope.Cluster.Spec.ControlPlaneEndpoint.Host,
		JoinToken:            token,
		JoinTokenTTLInSecs:   microk8sConfig.Spec.InitConfiguration.JoinTokenTTLInSecs,
		PortOfNodeToJoin:     portOfNodeToConnectTo,
		PortOfDqlite:         portOfDqlite,
		IPOfNodeToJoin:       ipOfNodeToConnectTo,
		Version:              *machine.Spec.Version,
	}
	if microk8sConfig.Spec.InitConfiguration != nil {
		controlPlaneInput.HTTPSProxy = microk8sConfig.Spec.InitConfiguration.HTTPSProxy
		controlPlaneInput.HTTPProxy = microk8sConfig.Spec.InitConfiguration.HTTPProxy
		controlPlaneInput.NoProxy = microk8sConfig.Spec.InitConfiguration.NoProxy

		if microk8sConfig.Spec.InitConfiguration.IPinIP {
			controlPlaneInput.IPinIP = true
		}

		if microk8sConfig.Spec.InitConfiguration.JoinTokenTTLInSecs == 0 {
			// set by default to 10 years
			controlPlaneInput.JoinTokenTTLInSecs = 315569260
		}
	}

	bootstrapInitData, err := cloudinit.NewJoinControlPlane(controlPlaneInput)

	if err != nil {
		scope.Error(err, "Failed to generate user data for bootstrap control plane")
		return ctrl.Result{}, err
	}

	if err := r.storeBootstrapData(ctx, scope, bootstrapInitData); err != nil {
		scope.Error(err, "Failed to store bootstrap data")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MicroK8sConfigReconciler) handleJoiningWorkerNode(ctx context.Context, scope *Scope) (_ ctrl.Result, reterr error) {
	// if it's a control plane machine, requeue
	if scope.ConfigOwner.IsControlPlaneMachine() {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	machine := &clusterv1.Machine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(scope.ConfigOwner.Object, machine); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "cannot convert %s to Machine", scope.ConfigOwner.GetKind())
	}

	// acquire the init lock so that only only one machine joins each time
	if !r.MicroK8sInitLock.Lock(ctx, scope.Cluster, machine) {
		scope.Info("A node is already being handled, requeueing until cluster can be extended with this node")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	defer func() {
		if reterr != nil {
			if !r.MicroK8sInitLock.Unlock(ctx, scope.Cluster) {
				reterr = kerrors.NewAggregate([]error{reterr, errors.New("failed to unlock the init lock")})
			}
		}
	}()

	scope.Info("Creating BootstrapData for the joining worker")

	microk8sConfig := &bootstrapclusterxk8siov1beta1.MicroK8sConfig{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: machine.Spec.Bootstrap.ConfigRef.Namespace,
		Name:      machine.Spec.Bootstrap.ConfigRef.Name,
	}, microk8sConfig); err != nil {
		return ctrl.Result{}, err
	}

	portOfNodeToConnectTo := defaultClusterAgentPort
	if microk8sConfig.Spec.ClusterConfiguration == nil ||
		microk8sConfig.Spec.ClusterConfiguration.PortCompatibilityRemap {
		portOfNodeToConnectTo = remappedClusterAgentPort
	}

	ipOfNodeToConnectTo, err := r.getControlPlaneNodeToJoin(ctx, scope)
	if err != nil || ipOfNodeToConnectTo == "" {
		scope.Info("Failed to discover a control plane IP, requeueing.")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	token, err := r.getJoinToken(ctx, scope)
	if err != nil {
		scope.Info("Failed to get or generate the join token, requeueing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	workerInput := &cloudinit.WorkerJoinInput{
		BaseUserData:         cloudinit.BaseUserData{},
		JoinToken:            token,
		ControlPlaneEndpoint: scope.Cluster.Spec.ControlPlaneEndpoint.Host,
		PortOfNodeToJoin:     portOfNodeToConnectTo,
		IPOfNodeToJoin:       ipOfNodeToConnectTo,
		Version:              *machine.Spec.Version,
	}

	if microk8sConfig.Spec.InitConfiguration != nil {
		workerInput.HTTPSProxy = microk8sConfig.Spec.InitConfiguration.HTTPSProxy
		workerInput.HTTPProxy = microk8sConfig.Spec.InitConfiguration.HTTPProxy
		workerInput.NoProxy = microk8sConfig.Spec.InitConfiguration.NoProxy
	}

	bootstrapInitData, err := cloudinit.NewJoinWorker(workerInput)

	if err != nil {
		scope.Error(err, "Failed to generate user data for bootstrap control plane")
		return ctrl.Result{}, err
	}

	if err := r.storeBootstrapData(ctx, scope, bootstrapInitData); err != nil {
		scope.Error(err, "Failed to store bootstrap data")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MicroK8sConfigReconciler) getControlPlaneNodeToJoin(ctx context.Context, scope *Scope) (string, error) {
	nodes, err := r.getControlPlaneMachinesForCluster(ctx, util.ObjectKey(scope.Cluster))
	if err != nil {
		scope.Error(err, "Lookup control plane nodes")
		return "", err
	}
	ipOfNodeToConnectTo := ""
	for _, node := range nodes {
		if ipOfNodeToConnectTo != "" {
			break
		}
		if node.Spec.ProviderID != nil && node.Status.Phase == "Running" {
			for _, address := range node.Status.Addresses {
				if address.Address != "" {
					ipOfNodeToConnectTo = address.Address
					break
				}
			}
		}
	}
	return ipOfNodeToConnectTo, nil
}

func (r *MicroK8sConfigReconciler) storeBootstrapData(ctx context.Context, scope *Scope, data []byte) error {
	log := ctrl.LoggerFrom(ctx)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scope.Config.Name,
			Namespace: scope.Config.Namespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: scope.Cluster.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: bootstrapclusterxk8siov1beta1.GroupVersion.String(),
					Kind:       "MicroK8sConfig",
					Name:       scope.Config.Name,
					UID:        scope.Config.UID,
					Controller: pointer.BoolPtr(true),
				},
			},
		},
		Data: map[string][]byte{
			"value":  data,
			"format": []byte("cloud-config"),
		},
		Type: clusterv1.ClusterSecretType,
	}

	// as secret creation and scope.Config status patch are not atomic operations
	// it is possible that secret creation happens but the config.Status patches are not applied
	if err := r.Client.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create bootstrap data secret for MicroK8sConfig %s/%s", scope.Config.Namespace, scope.Config.Name)
		}
		log.Info("bootstrap data secret for MicroK8sConfig already exists, updating", "secret", secret.Name, "MicroK8sConfig", scope.Config.Name)
		if err := r.Client.Update(ctx, secret); err != nil {
			return errors.Wrapf(err, "failed to update bootstrap data secret for MicroK8sConfig %s/%s", scope.Config.Namespace, scope.Config.Name)
		}
	}
	scope.Config.Status.DataSecretName = pointer.StringPtr(secret.Name)
	scope.Config.Status.Ready = true
	conditions.MarkTrue(scope.Config, bootstrapclusterxk8siov1beta1.DataSecretAvailableCondition)
	return nil
}

func (r *MicroK8sConfigReconciler) getJoinToken(ctx context.Context, scope *Scope) (string, error) {
	// See if the token exists. If not create it.
	secrets := &corev1.SecretList{}
	err := r.Client.List(ctx, secrets)
	if err != nil {
		return "", err
	}

	found := false
	for _, s := range secrets.Items {
		if s.Name == scope.Cluster.Name+"-jointoken" {
			found = true
		}
	}

	if !found {
		const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		b := make([]byte, 32)
		for i := range b {
			b[i] = letters[mrand.Intn(len(letters))]
		}
		token := string(b)
		tokenSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: scope.Cluster.Namespace,
				Name:      scope.Cluster.Name + "-jointoken",
			},
			Data: map[string][]byte{
				"value": []byte(token),
			},
		}
		err = r.Client.Create(ctx, tokenSecret)
		if err != nil {
			return "", err
		}
	}

	readTokenSecret := &corev1.Secret{}
	err = r.Client.Get(ctx,
		types.NamespacedName{
			Namespace: scope.Cluster.Namespace,
			Name:      scope.Cluster.Name + "-jointoken",
		},
		readTokenSecret,
	)
	if err != nil {
		return "", err
	}

	return string(readTokenSecret.Data["value"]), nil
}

func (r *MicroK8sConfigReconciler) getCA(ctx context.Context, scope *Scope) (cert *string, key *string, err error) {
	// See if the CA cert exists. If not create it.
	secrets := &corev1.SecretList{}
	err = r.Client.List(ctx, secrets)
	if err != nil {
		return nil, nil, err
	}

	found := false
	for _, s := range secrets.Items {
		if s.Name == scope.Cluster.Name+"-ca" {
			found = true
		}
	}

	if !found {
		newcrt, newkey, err := r.generateCA()
		if err != nil {
			return nil, nil, err
		}
		caSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: scope.Cluster.Namespace,
				Name:      scope.Cluster.Name + "-ca",
			},
			Data: map[string][]byte{
				"crt": []byte(*newcrt),
				"key": []byte(*newkey),
			},
		}
		err = r.Client.Create(ctx, caSecret)
		if err != nil {
			return nil, nil, err
		}
	}

	readCASecret := &corev1.Secret{}
	err = r.Client.Get(ctx,
		types.NamespacedName{
			Namespace: scope.Cluster.Namespace,
			Name:      scope.Cluster.Name + "-ca",
		},
		readCASecret,
	)
	if err != nil {
		return nil, nil, err
	}

	certstr := string(readCASecret.Data["crt"])
	keystr := string(readCASecret.Data["key"])
	return &certstr, &keystr, nil
}

func (r *MicroK8sConfigReconciler) generateCA() (cert *string, key *string, err error) {
	// set up our CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Canonical"},
			Country:       []string{"GB"},
			Province:      []string{""},
			Locality:      []string{"Canonical"},
			StreetAddress: []string{"Canonical"},
			CommonName:    "10.152.183.1",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return nil, nil, err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err != nil {
		return nil, nil, err
	}

	certstr := caPEM.String()
	keystr := caPrivKeyPEM.String()
	return &certstr, &keystr, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MicroK8sConfigReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	if r.MicroK8sInitLock == nil {
		r.MicroK8sInitLock = locking.NewControlPlaneInitMutex(ctrl.LoggerFrom(ctx).WithName("init-locker"), mgr.GetClient())
	}

	b := ctrl.NewControllerManagedBy(mgr).
		For(&bootstrapclusterxk8siov1beta1.MicroK8sConfig{}).
		Watches(&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(r.MachineToBootstrapMapFunc)).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx),
			r.WatchFilterValue))

	if feature.Gates.Enabled(feature.MachinePool) {
		b = b.Watches(
			&source.Kind{Type: &expv1.MachinePool{}},
			handler.EnqueueRequestsFromMapFunc(r.MachineToBootstrapMapFunc),
		).WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue))
	}

	c, err := b.Build(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}
	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.ClusterToMicroK8sConfigs),
		predicates.All(ctrl.LoggerFrom(ctx),
			predicates.ClusterUnpausedAndInfrastructureReady(ctrl.LoggerFrom(ctx)),
			predicates.ResourceHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue),
		),
	)
	if err != nil {
		return errors.Wrap(err, "failed adding Watch for Clusters to controller manager")
	}

	return nil
}

func (r *MicroK8sConfigReconciler) ClusterToMicroK8sConfigs(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}

	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	selectors := []client.ListOption{
		client.InNamespace(c.Namespace),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: c.Name,
		},
	}

	machineList := &clusterv1.MachineList{}
	if err := r.Client.List(context.TODO(), machineList, selectors...); err != nil {
		return nil
	}

	for _, m := range machineList.Items {
		if m.Spec.Bootstrap.ConfigRef != nil &&
			m.Spec.Bootstrap.ConfigRef.GroupVersionKind().GroupKind() == v1beta1.GroupVersion.WithKind("MicroK8sConfig").GroupKind() {
			name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.Bootstrap.ConfigRef.Name}
			result = append(result, ctrl.Request{NamespacedName: name})
		}
	}

	if feature.Gates.Enabled(feature.MachinePool) {
		machinePoolList := &expv1.MachinePoolList{}
		if err := r.Client.List(context.TODO(), machinePoolList, selectors...); err != nil {
			return nil
		}

		for _, mp := range machinePoolList.Items {
			if mp.Spec.Template.Spec.Bootstrap.ConfigRef != nil &&
				mp.Spec.Template.Spec.Bootstrap.ConfigRef.GroupVersionKind().GroupKind() == v1beta1.GroupVersion.WithKind("MicroK8sConfig").GroupKind() {
				name := client.ObjectKey{Namespace: mp.Namespace, Name: mp.Spec.Template.Spec.Bootstrap.ConfigRef.Name}
				result = append(result, ctrl.Request{NamespacedName: name})
			}
		}
	}

	return result
}

func (r *MicroK8sConfigReconciler) MachineToBootstrapMapFunc(o client.Object) []ctrl.Request {
	m, ok := o.(*clusterv1.Machine)
	if !ok {
		panic(fmt.Sprintf("Expected a Machine but got a %T", o))
	}

	result := []ctrl.Request{}
	if m.Spec.Bootstrap.ConfigRef != nil && m.Spec.Bootstrap.ConfigRef.GroupVersionKind() == v1beta1.GroupVersion.WithKind("MicroK8sConfig") {
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.Bootstrap.ConfigRef.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}
	return result
}

func (r *MicroK8sConfigReconciler) getControlPlaneMachinesForCluster(ctx context.Context,
	cluster client.ObjectKey) ([]clusterv1.Machine, error) {
	selector := map[string]string{
		clusterv1.ClusterLabelName:             cluster.Name,
		clusterv1.MachineControlPlaneLabelName: "",
	}

	machineList := clusterv1.MachineList{}
	if err := r.Client.List(
		ctx,
		&machineList,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(selector),
	); err != nil {
		return nil, err
	}

	return machineList.Items, nil
}
