// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
)

var (
	// Scheme is a Scheme with the types relevant for Azure actuators.
	Scheme *runtime.Scheme

	decoder runtime.Decoder

	// lenientDecoder is a decoder that does not use strict mode.
	lenientDecoder runtime.Decoder

	// InfrastructureStateTypeMeta is the TypeMeta of the Azure InfrastructureStatus
	InfrastructureStateTypeMeta = metav1.TypeMeta{
		APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
		Kind:       "InfrastructureState",
	}
)

func init() {
	Scheme = runtime.NewScheme()
	utilruntime.Must(install.AddToScheme(Scheme))

	decoder = serializer.NewCodecFactory(Scheme, serializer.EnableStrict).UniversalDecoder()
	lenientDecoder = serializer.NewCodecFactory(Scheme).UniversalDecoder()
}

// InfrastructureConfigFromInfrastructure extracts the InfrastructureConfig from the
// ProviderConfig section of the given Infrastructure.
func InfrastructureConfigFromInfrastructure(infra *extensionsv1alpha1.Infrastructure) (*api.InfrastructureConfig, error) {
	config := &api.InfrastructureConfig{}
	if infra.Spec.ProviderConfig != nil && infra.Spec.ProviderConfig.Raw != nil {
		if _, _, err := decoder.Decode(infra.Spec.ProviderConfig.Raw, nil, config); err != nil {
			return nil, err
		}
		return config, nil
	}
	return nil, fmt.Errorf("provider config is not set on the infrastructure resource")
}

// InfrastructureStatusFromRaw extracts the InfrastructureStatus from the
// ProviderStatus section of the given Infrastructure.
func InfrastructureStatusFromRaw(raw *runtime.RawExtension) (*api.InfrastructureStatus, error) {
	status := &api.InfrastructureStatus{}
	if raw != nil && raw.Raw != nil {
		if _, _, err := lenientDecoder.Decode(raw.Raw, nil, status); err != nil {
			return nil, err
		}
		return status, nil
	}
	return nil, fmt.Errorf("provider status is not set on the infrastructure resource")
}

// CloudProfileConfigFromCluster decodes the provider specific cloud profile configuration for a cluster
func CloudProfileConfigFromCluster(cluster *controller.Cluster) (*api.CloudProfileConfig, error) {
	var cloudProfileConfig *api.CloudProfileConfig
	if cluster != nil && cluster.CloudProfile != nil && cluster.CloudProfile.Spec.ProviderConfig != nil && cluster.CloudProfile.Spec.ProviderConfig.Raw != nil {
		cloudProfileSpecifier := fmt.Sprintf("CloudProfile %q", k8sclient.ObjectKeyFromObject(cluster.CloudProfile))
		if cluster.Shoot != nil && cluster.Shoot.Spec.CloudProfile != nil {
			cloudProfileSpecifier = fmt.Sprintf("%s '%s/%s'", cluster.Shoot.Spec.CloudProfile.Kind, cluster.Shoot.Namespace, cluster.Shoot.Spec.CloudProfile.Name)
		}
		cloudProfileConfig = &api.CloudProfileConfig{}
		if _, _, err := decoder.Decode(cluster.CloudProfile.Spec.ProviderConfig.Raw, nil, cloudProfileConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of %s: %w", cloudProfileSpecifier, err)
		}
	}
	return cloudProfileConfig, nil
}

// BackupConfigFromBackupBucket decodes the provider specific config from a given BackupBucket object.
func BackupConfigFromBackupBucket(backupBucket *extensionsv1alpha1.BackupBucket) (api.BackupBucketConfig, error) {
	if backupBucket == nil || backupBucket.Spec.ProviderConfig == nil {
		return api.BackupBucketConfig{}, nil
	}

	return BackupConfigFromProviderConfig(backupBucket.Spec.ProviderConfig)
}

// BackupConfigFromProviderConfig decodes the provider specific config from the RawExtension.
func BackupConfigFromProviderConfig(config *runtime.RawExtension) (api.BackupBucketConfig, error) {
	backupConfig := api.BackupBucketConfig{}

	bucketJson, err := config.MarshalJSON()
	if err != nil {
		return backupConfig, err
	}

	_, _, err = decoder.Decode(bucketJson, nil, &backupConfig)

	return backupConfig, err
}

// InfrastructureStateFromRaw extracts the state from the Infrastructure. If no state was available, it returns a "zero" value InfrastructureState object.
func InfrastructureStateFromRaw(raw *runtime.RawExtension) (*api.InfrastructureState, error) {
	state := &api.InfrastructureState{}
	if raw != nil && (len(raw.Raw) > 0 || raw.Object != nil) {
		data, err := raw.MarshalJSON()
		if err != nil {
			return nil, err
		}
		obj, _, err := lenientDecoder.Decode(data, nil, nil)
		if err != nil {
			return nil, err
		}
		var ok bool
		if state, ok = obj.(*api.InfrastructureState); !ok {
			return nil, fmt.Errorf("expected InfrastructureState, got %T", obj)
		}
	}

	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	if state.ManagedItems == nil {
		state.ManagedItems = make([]api.AzureResource, 0)
	}

	return state, nil
}

// InfrastructureStatusFromInfrastructure extracts the InfrastructureStatus from the
// ProviderStatus section of the given Infrastructure.
// If the providerConfig is missing from the status, it will return a zero-value InfrastructureStatus.
func InfrastructureStatusFromInfrastructure(infra *extensionsv1alpha1.Infrastructure) (*api.InfrastructureStatus, error) {
	status := &api.InfrastructureStatus{}
	if infra.Status.ProviderStatus != nil && infra.Status.ProviderStatus.Raw != nil {
		return InfrastructureStatusFromRaw(infra.Status.ProviderStatus)
	}
	return status, nil
}

// WorkloadIdentityConfigFromRaw extracts WorkloadIdentityConfig from the provided [runtime.RawExtension].
func WorkloadIdentityConfigFromRaw(raw *runtime.RawExtension) (*api.WorkloadIdentityConfig, error) {
	if raw == nil || raw.Raw == nil {
		return nil, errors.New("cannot parse WorkloadIdentityConfig from empty RawExtension")
	}
	return WorkloadIdentityConfigFromBytes(raw.Raw)
}

// WorkloadIdentityConfigFromBytes extracts WorkloadIdentityConfig from the provided byte array.
func WorkloadIdentityConfigFromBytes(config []byte) (*api.WorkloadIdentityConfig, error) {
	if len(config) == 0 {
		return nil, fmt.Errorf("cannot parse WorkloadIdentityConfig from empty config")
	}
	workloadIdentityConfig := &api.WorkloadIdentityConfig{}
	if err := util.Decode(decoder, config, workloadIdentityConfig); err != nil {
		return nil, err
	}
	return workloadIdentityConfig, nil
}
