package v1beta1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// Conditions and condition Reasons for the KubeadmConfig object.

const (
	// DataSecretAvailableCondition documents the status of the bootstrap secret generation process.
	//
	// NOTE: When the DataSecret generation starts the process completes immediately and within the
	// same reconciliation, so the user will always see a transition from Wait to Generated without having
	// evidence that BootstrapSecret generation is started/in progress.
	DataSecretAvailableCondition clusterv1.ConditionType = "DataSecretAvailable"

	// WaitingForClusterInfrastructureReason (Severity=Info) document a bootstrap secret generation process
	// waiting for the cluster infrastructure to be ready.
	//
	// NOTE: Having the cluster infrastructure ready is a pre-condition for starting to create machines;
	// the KubeadmConfig controller ensure this pre-condition is satisfied.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"

	// DataSecretGenerationFailedReason (Severity=Warning) documents a KubeadmConfig controller detecting
	// an error while generating a data secret; those kind of errors are usually due to misconfigurations
	// and user intervention is required to get them fixed.
	DataSecretGenerationFailedReason = "DataSecretGenerationFailed"
)

const (
	// CertificatesAvailableCondition documents that cluster certificates are available.
	//
	// NOTE: Cluster certificates are generated only for the KubeadmConfig object linked to the initial control plane
	// machine, if the cluster is not using a control plane ref object, if the certificates are not provided
	// by the users.
	// IMPORTANT: This condition won't be re-created after clusterctl move.
	CertificatesAvailableCondition clusterv1.ConditionType = "CertificatesAvailable"
)
