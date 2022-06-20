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
	"context"
	"fmt"
	"time"

	bootstrapclusterxk8siov1beta1 "cluster-api-bootstrap-provider-microk8s/apis/v1beta1"
	cloudinit "cluster-api-bootstrap-provider-microk8s/controllers/cloudinit"

	"cluster-api-bootstrap-provider-microk8s/apis/v1beta1"

	"cluster-api-bootstrap-provider-microk8s/controllers/locking"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=microk8sconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=microk8sconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=microk8sconfigs/finalizers,verbs=update

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
		if errors.Cause(err) == util.ErrNoCluster {
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
		// if config.Spec.JoinConfiguration != nil && config.Spec.JoinConfiguration.Discovery.BootstrapToken != nil {
		// 	if !configOwner.IsInfrastructureReady() {
		// 		// If the BootstrapToken has been generated for a join and the infrastructure is not ready.
		// 		// This indicates the token in the join config has not been consumed and it may need a refresh.
		// 		//	return r.refreshBootstrapToken(ctx, config, cluster)
		// 	}
		// 	if configOwner.IsMachinePool() {
		// 		// If the BootstrapToken has been generated and infrastructure is ready but the configOwner is a MachinePool,
		// 		// we rotate the token to keep it fresh for future scale ups.
		// 		//		return r.rotateMachinePoolBootstrapToken(ctx, config, cluster, scope)
		// 	}
		// }
		// In any other case just return as the config is already generated and need not be generated again.
		return ctrl.Result{}, nil
	}

	// Note: can't use IsFalse here because we need to handle the absence of the condition as well as false.
	if !conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
		log.Info("Cluster control plane is not initialized, waiting")
		return r.handleClusterNotInitialized(ctx, scope)
	}

	// Every other case it's a join scenario
	// Nb. in this case ClusterConfiguration and InitConfiguration should not be defined by users, but in case of misconfigurations, CABPK simply ignore them

	// Unlock any locks that might have been set during init process
	r.MicroK8sInitLock.Unlock(ctx, cluster)

	// if the JoinConfiguration is missing, create a default one
	if config.Spec.JoinConfiguration == nil {
		log.Info("Creating default JoinConfiguration")
		//	config.Spec.JoinConfiguration = &bootstrapv1.JoinConfiguration{}
	}

	// it's a control plane join
	if configOwner.IsControlPlaneMachine() {
		log.Info("Reconciling control plane")
	}

	// It's a worker join
	return ctrl.Result{}, nil
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

	// if the machine has not ClusterConfiguration and InitConfiguration, requeue
	if scope.Config.Spec.InitConfiguration == nil && scope.Config.Spec.ClusterConfiguration == nil {
		scope.Info("Control plane is not ready, requeing joining control planes until ready.")
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
		scope.Info("A control plane is already being initialized, requeing until control plane is ready")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	defer func() {
		if reterr != nil {
			if !r.MicroK8sInitLock.Unlock(ctx, scope.Cluster) {
				reterr = kerrors.NewAggregate([]error{reterr, errors.New("failed to unlock the kubeadm init lock")})
			}
		}
	}()

	scope.Info("Creating BootstrapData for the init control plane")

	// Nb. in this case JoinConfiguration should not be defined by users, but in case of misconfigurations, CABPK simply ignore it

	// get both of ClusterConfiguration and InitConfiguration strings to pass to the cloud init control plane generator
	// kubeadm allows one of these values to be empty; CABPK replace missing values with an empty config, so the cloud init generation
	// should not handle special cases.

	// kubernetesVersion := scope.ConfigOwner.KubernetesVersion()
	// parsedVersion, err := semver.ParseTolerant(kubernetesVersion)
	// if err != nil {
	// 	return ctrl.Result{}, errors.Wrapf(err, "failed to parse kubernetes version %q", kubernetesVersion)
	// }

	// if scope.Config.Spec.InitConfiguration == nil {
	// 	scope.Config.Spec.InitConfiguration = &bootstrapclusterxk8siov1beta1.InitConfiguration{
	// 		TypeMeta: metav1.TypeMeta{
	// 			APIVersion: "kubeadm.k8s.io/v1beta1",
	// 			Kind:       "InitConfiguration",
	// 		},
	// 	}
	// }
	// initdata, err := kubeadmtypes.MarshalInitConfigurationForVersion(scope.Config.Spec.InitConfiguration, parsedVersion)
	// if err != nil {
	// 	scope.Error(err, "Failed to marshal init configuration")
	// 	return ctrl.Result{}, err
	// }

	// if scope.Config.Spec.ClusterConfiguration == nil {
	// 	scope.Config.Spec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{
	// 		TypeMeta: metav1.TypeMeta{
	// 			APIVersion: "kubeadm.k8s.io/v1beta1",
	// 			Kind:       "ClusterConfiguration",
	// 		},
	// 	}
	// }

	// // injects into config.ClusterConfiguration values from top level object
	// r.reconcileTopLevelObjectSettings(ctx, scope.Cluster, machine, scope.Config)

	// clusterdata, err := kubeadmtypes.MarshalClusterConfigurationForVersion(scope.Config.Spec.ClusterConfiguration, parsedVersion)
	// if err != nil {
	// 	scope.Error(err, "Failed to marshal cluster configuration")
	// 	return ctrl.Result{}, err
	// }

	// certificates := secret.NewCertificatesForInitialControlPlane(scope.Config.Spec.ClusterConfiguration)
	// err = certificates.LookupOrGenerate(
	// 	ctx,
	// 	r.Client,
	// 	util.ObjectKey(scope.Cluster),
	// 	*metav1.NewControllerRef(scope.Config, bootstrapv1.GroupVersion.WithKind("KubeadmConfig")),
	// )
	// if err != nil {
	// 	conditions.MarkFalse(scope.Config, bootstrapv1.CertificatesAvailableCondition, bootstrapv1.CertificatesGenerationFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
	// 	return ctrl.Result{}, err
	// }
	// conditions.MarkTrue(scope.Config, bootstrapv1.CertificatesAvailableCondition)

	// verbosityFlag := ""
	// if scope.Config.Spec.Verbosity != nil {
	// 	verbosityFlag = fmt.Sprintf("--v %s", strconv.Itoa(int(*scope.Config.Spec.Verbosity)))
	// }

	// files, err := r.resolveFiles(ctx, scope.Config)
	// if err != nil {
	// 	conditions.MarkFalse(scope.Config, bootstrapv1.DataSecretAvailableCondition, bootstrapv1.DataSecretGenerationFailedReason, clusterv1.ConditionSeverityWarning, err.Error())
	// 	return ctrl.Result{}, err
	// }

	controlPlaneInput := &cloudinit.ControlPlaneInput{
		BaseUserData: cloudinit.BaseUserData{
			//	AdditionalFiles:     files,
			//	NTP:                 scope.Config.Spec.NTP,
			//	PreKubeadmCommands:  scope.Config.Spec.PreKubeadmCommands,
			//	PostKubeadmCommands: scope.Config.Spec.PostKubeadmCommands,
			//	Users:               scope.Config.Spec.Users,
			//	Mounts:              scope.Config.Spec.Mounts,
			//	DiskSetup:           scope.Config.Spec.DiskSetup,
			//	KubeadmVerbosity:    verbosityFlag,
		},
		ControlPlaneEndpoint: scope.Cluster.Spec.ControlPlaneEndpoint.Host,
		//	InitConfiguration:    initdata,
		//	ClusterConfiguration: clusterdata,
		//	Certificates:         certificates,
	}

	// var bootstrapInitData []byte
	// switch scope.Config.Spec.Format {
	// case bootstrapv1.Ignition:
	// 	bootstrapInitData, _, err = ignition.NewInitControlPlane(&ignition.ControlPlaneInput{
	// 		ControlPlaneInput: controlPlaneInput,
	// 		Ignition:          scope.Config.Spec.Ignition,
	// 	})
	// default:
	bootstrapInitData, err := cloudinit.NewInitControlPlane(controlPlaneInput)
	// }

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
