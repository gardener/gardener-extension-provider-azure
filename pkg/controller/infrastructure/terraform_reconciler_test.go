// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	mockterraform "github.com/gardener/gardener/extensions/pkg/terraformer/mock"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	azureclientmocks "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	. "github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

const (
	name                       = "az"
	namespace                  = "shoot--foobar--az"
	region                     = "region"
	disableProjectedTokenMount = false
)

var _ = Describe("Actuator", func() {
	var (
		ctrl               *gomock.Controller
		c                  *mockclient.MockClient
		mgr                *mockmanager.MockManager
		sw                 *mockclient.MockStatusWriter
		log                logr.Logger
		tf                 *mockterraform.MockTerraformer
		ctx                context.Context
		a                  infrastructure.Actuator
		infra              *extensionsv1alpha1.Infrastructure
		providerConfig     *api.InfrastructureConfig
		cluster            *extensions.Cluster
		cloudProfileConfig *api.CloudProfileConfig
		providerStatus     *apiv1alpha1.InfrastructureStatus
		tfState            *terraformer.RawState
		revert             func()

		err error
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		tf = mockterraform.NewMockTerraformer(ctrl)
		c.EXPECT().Status().Return(sw).AnyTimes()

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)
		mgr.EXPECT().GetConfig().Return(&rest.Config{})

		ctx = context.TODO()
		log = logf.Log.WithName("test")

		a = NewActuator(mgr, disableProjectedTokenMount)

		providerConfig = &api.InfrastructureConfig{
			Networks: api.NetworkConfig{
				VNet: api.VNet{
					CIDR: ptr.To("10.222.0.0/16"),
				},
				Workers: ptr.To("10.222.0.0/16"),
			},
			Zoned: true,
		}

		infra, err = createInfra(providerConfig)
		Expect(err).NotTo(HaveOccurred())

		cloudProfileConfig = &api.CloudProfileConfig{
			CountFaultDomains: []api.DomainCount{
				{
					Region: region,
					Count:  3,
				},
			},
			CountUpdateDomains: []api.DomainCount{
				{
					Region: region,
					Count:  5,
				},
			},
		}

		cluster, err = createCluster(cloudProfileConfig)
		Expect(err).NotTo(HaveOccurred())

		tfState = &terraformer.RawState{Data: `{"foo":"bar"}`}

		providerStatus = &apiv1alpha1.InfrastructureStatus{
			TypeMeta: infrainternal.StatusTypeMeta,
			ResourceGroup: apiv1alpha1.ResourceGroup{
				Name: "",
			},
			Networks: apiv1alpha1.NetworkStatus{
				Layout: "SingleSubnet",
				VNet: apiv1alpha1.VNetStatus{
					Name: "",
				},
				OutboundAccessType: apiv1alpha1.OutboundAccessTypeLoadBalancer,
			},
			AvailabilitySets: []apiv1alpha1.AvailabilitySet{},
			RouteTables: []apiv1alpha1.RouteTable{
				{Purpose: apiv1alpha1.PurposeNodes, Name: ""},
			},
			SecurityGroups: []apiv1alpha1.SecurityGroup{
				{Name: "", Purpose: apiv1alpha1.PurposeNodes},
			},
			Zoned: true,
		}

		revert = test.WithVars(
			&internal.NewTerraformer, func(_ logr.Logger, _ *rest.Config, _ string, _ *extensionsv1alpha1.Infrastructure, _ bool) (terraformer.Terraformer, error) {
				return tf, nil
			},
			&internal.NewTerraformerWithAuth, func(_ logr.Logger, _ *rest.Config, _ string, _ *extensionsv1alpha1.Infrastructure, _ bool, _ bool) (terraformer.Terraformer, error) {
				return tf, nil
			},
		)
	})

	AfterEach(func() {
		revert()
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		It("should reconcile the Infrastructure", func() {
			tf.EXPECT().InitializeWith(ctx, gomock.Any()).Return(tf)
			tf.EXPECT().Apply(ctx)
			tf.EXPECT().GetStateOutputVariables(ctx, gomock.Any())
			tf.EXPECT().GetRawState(ctx).Return(tfState, nil)

			state, err := createInfraState(providerStatus, tfState)
			Expect(err).NotTo(HaveOccurred())
			expectedInfra := infra.DeepCopy()
			expectedInfra.Status.ProviderStatus = &runtime.RawExtension{Object: providerStatus}
			expectedInfra.Status.State = state

			test.EXPECTStatusPatch(ctx, sw, expectedInfra, infra.DeepCopy(), types.MergePatchType)

			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)
			err = a.Reconcile(ctx, log, infra, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if infrastructure provider config does not exist", func() {
			infra.Spec.ProviderConfig = nil
			err := a.Reconcile(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if infrastructure provider config is not properly set", func() {
			infra.Spec.ProviderConfig.Raw = nil
			err := a.Reconcile(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if terraformer cannot be created", func() {
			defer test.WithVars(
				&internal.NewTerraformerWithAuth, func(_ logr.Logger, _ *rest.Config, _ string, _ *extensionsv1alpha1.Infrastructure, _ bool, _ bool) (terraformer.Terraformer, error) {
					return nil, errors.New("could not create terraform")
				},
			)()
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)
			err := a.Reconcile(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if terraform config cannot be applied", func() {
			tf.EXPECT().InitializeWith(ctx, gomock.Any()).Return(tf)
			tf.EXPECT().Apply(ctx).Return(errors.New("could not apply terraform config"))
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)
			err := a.Reconcile(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#Delete", func() {
		var (
			azureClientFactory *azureclientmocks.MockFactory
			azureGroupClient   *azureclientmocks.MockResourceGroup
			resourceGroupName  string
		)

		BeforeEach(func() {
			azureClientFactory = azureclientmocks.NewMockFactory(ctrl)
			azureGroupClient = azureclientmocks.NewMockResourceGroup(ctrl)
			resourceGroupName = infra.Namespace

			DefaultAzureClientFactoryFunc = func(context.Context, client.Client, v1.SecretReference, bool, ...azureclient.AzureFactoryOption) (azureclient.Factory, error) {
				return azureClientFactory, nil
			}
		})

		It("should delete the Infrastructure", func() {
			azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil).Times(2)
			azureGroupClient.EXPECT().Get(ctx, infra.Namespace).Return(&armresources.ResourceGroup{Name: &resourceGroupName}, nil)
			azureGroupClient.EXPECT().Delete(ctx, infra.Namespace).Return(nil)

			tf.EXPECT().EnsureCleanedUp(ctx)
			tf.EXPECT().IsStateEmpty(ctx).Return(false)
			tf.EXPECT().InitializeWith(ctx, gomock.Any()).Return(tf)

			envVars := internal.TerraformerEnvVars(infra.Spec.SecretRef, false)
			tf.EXPECT().SetEnvVars(envVars).Return(tf)
			tf.EXPECT().Destroy(ctx)
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)
			err := a.Delete(ctx, log, infra, cluster)
			Expect(err).NotTo(HaveOccurred())

		})

		It("should delete the Infrastructure with invalid credentials", func() {
			azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
			azureGroupClient.EXPECT().Get(ctx, infra.Namespace).Return(nil, autorest.DetailedError{Response: &http.Response{StatusCode: http.StatusUnauthorized}})

			tf.EXPECT().EnsureCleanedUp(ctx)
			tf.EXPECT().RemoveTerraformerFinalizerFromConfig(gomock.Any()).Return(nil)
			tf.EXPECT().CleanupConfiguration(ctx).Return(nil)

			err := a.Delete(ctx, log, infra, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should exit early if the Infrastructure's terraform state is empty", func() {
			azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
			azureGroupClient.EXPECT().Get(ctx, infra.Namespace).Return(&armresources.ResourceGroup{Name: &resourceGroupName}, nil)

			tf.EXPECT().EnsureCleanedUp(ctx)
			tf.EXPECT().IsStateEmpty(ctx).Return(true)
			tf.EXPECT().CleanupConfiguration(ctx)
			err := a.Delete(ctx, log, infra, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if terraformer cannot be created", func() {
			defer test.WithVars(
				&internal.NewTerraformer, func(_ logr.Logger, _ *rest.Config, _ string, _ *extensionsv1alpha1.Infrastructure, _ bool) (terraformer.Terraformer, error) {
					return nil, errors.New("could not create terraform")
				},
			)()
			err := a.Delete(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if running terraform pod cannot be cleaned up", func() {
			tf.EXPECT().EnsureCleanedUp(ctx).Return(errors.New("could not clean up"))
			err := a.Delete(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if terraform state is empty and cleaning up the terraform config fails", func() {
			azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
			azureGroupClient.EXPECT().Get(ctx, infra.Namespace).Return(&armresources.ResourceGroup{Name: &resourceGroupName}, nil)

			tf.EXPECT().EnsureCleanedUp(ctx)
			tf.EXPECT().IsStateEmpty(ctx).Return(true)
			tf.EXPECT().CleanupConfiguration(ctx).Return(errors.New("could not clean up terraform config"))
			err := a.Delete(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if terraform cannot be destroyed", func() {
			azureClientFactory.EXPECT().Group().Return(azureGroupClient, nil)
			azureGroupClient.EXPECT().Get(ctx, infra.Namespace).Return(&armresources.ResourceGroup{Name: &resourceGroupName}, nil)

			tf.EXPECT().EnsureCleanedUp(ctx)
			tf.EXPECT().IsStateEmpty(ctx).Return(false)
			tf.EXPECT().InitializeWith(ctx, gomock.Any()).Return(tf)
			envVars := internal.TerraformerEnvVars(infra.Spec.SecretRef, false)
			tf.EXPECT().SetEnvVars(envVars).Return(tf)
			tf.EXPECT().Destroy(ctx).Return(errors.New("could not destroy terraform"))
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)
			err := a.Delete(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#Restore", func() {
		It("should restore the Infrastructure", func() {
			state, err := createInfraState(providerStatus, tfState)
			Expect(err).NotTo(HaveOccurred())
			infra.Status.State = state

			expectedInfra := infra.DeepCopy()
			raw, err := json.Marshal(providerStatus)
			Expect(err).NotTo(HaveOccurred())
			expectedInfra.Status.ProviderStatus = &runtime.RawExtension{Raw: raw}
			test.EXPECTStatusPatch(ctx, sw, expectedInfra, infra, types.MergePatchType)

			tf.EXPECT().InitializeWith(ctx, gomock.Any()).Return(tf)
			tf.EXPECT().Apply(ctx)
			tf.EXPECT().GetStateOutputVariables(ctx, gomock.Any())
			tf.EXPECT().GetRawState(ctx).Return(tfState, nil)

			expectedInfraAfterRestore := expectedInfra.DeepCopy()
			expectedInfraAfterRestore.Status.ProviderStatus = &runtime.RawExtension{Object: providerStatus}
			test.EXPECTStatusPatch(ctx, sw, expectedInfraAfterRestore, expectedInfra.DeepCopy(), types.MergePatchType)
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)
			err = a.Restore(ctx, log, infra, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should restore infrastructure with countFault and countUpdate domain values found in ProviderStatus if availability set is required", func() {
			providerStatus.AvailabilitySets = append(providerStatus.AvailabilitySets, apiv1alpha1.AvailabilitySet{
				Purpose:            apiv1alpha1.PurposeNodes,
				CountFaultDomains:  ptr.To[int32](6),
				CountUpdateDomains: ptr.To[int32](7),
				ID:                 "id",
				Name:               "name",
			})
			providerStatus.Zoned = false
			providerConfig.Zoned = false

			infra, err := createInfra(providerConfig)
			Expect(err).NotTo(HaveOccurred())

			state, err := createInfraState(providerStatus, tfState)
			Expect(err).NotTo(HaveOccurred())
			infra.Status.State = state

			expectedInfra := infra.DeepCopy()
			raw, err := json.Marshal(providerStatus)
			Expect(err).NotTo(HaveOccurred())
			expectedInfra.Status.ProviderStatus = &runtime.RawExtension{Raw: raw}
			test.EXPECTStatusPatch(ctx, sw, expectedInfra, infra, types.MergePatchType)

			tf.EXPECT().InitializeWith(ctx, gomock.Any()).Return(tf)
			tf.EXPECT().Apply(ctx)
			tf.EXPECT().GetStateOutputVariables(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, variables ...string) (map[string]string, error) {
				Expect(variables).To(ContainElements(
					infrainternal.TerraformerOutputKeyCountFaultDomains,
					infrainternal.TerraformerOutputKeyCountUpdateDomains,
					infrainternal.TerraformerOutputKeyAvailabilitySetID,
					infrainternal.TerraformerOutputKeyAvailabilitySetName))
				output := map[string]string{}
				output[infrainternal.TerraformerOutputKeyCountFaultDomains] = "6"
				output[infrainternal.TerraformerOutputKeyCountUpdateDomains] = "7"
				output[infrainternal.TerraformerOutputKeyAvailabilitySetID] = "id"
				output[infrainternal.TerraformerOutputKeyAvailabilitySetName] = "name"

				return output, nil
			})
			tf.EXPECT().GetRawState(ctx).Return(tfState, nil)

			expectedInfraAfterRestore := expectedInfra.DeepCopy()
			expectedInfraAfterRestore.Status.ProviderStatus = &runtime.RawExtension{Object: providerStatus}
			test.EXPECTStatusPatch(ctx, sw, expectedInfraAfterRestore, expectedInfra.DeepCopy(), types.MergePatchType)
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)
			err = a.Restore(ctx, log, infra, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if terraformer cannot be created", func() {
			defer test.WithVars(
				&internal.NewTerraformerWithAuth, func(_ logr.Logger, _ *rest.Config, _ string, _ *extensionsv1alpha1.Infrastructure, _ bool, _ bool) (terraformer.Terraformer, error) {
					return nil, errors.New("could not create terraform")
				},
			)()
			state, err := createInfraState(providerStatus, tfState)
			Expect(err).NotTo(HaveOccurred())
			infra.Status.State = state

			expectedInfra := infra.DeepCopy()
			raw, err := json.Marshal(providerStatus)
			Expect(err).NotTo(HaveOccurred())
			expectedInfra.Status.ProviderStatus = &runtime.RawExtension{Raw: raw}
			test.EXPECTStatusPatch(ctx, sw, expectedInfra, infra, types.MergePatchType)
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)

			err = a.Restore(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if terraform config cannot be applied", func() {
			state, err := createInfraState(providerStatus, tfState)
			Expect(err).NotTo(HaveOccurred())
			infra.Status.State = state

			expectedInfra := infra.DeepCopy()
			raw, err := json.Marshal(providerStatus)
			Expect(err).NotTo(HaveOccurred())
			expectedInfra.Status.ProviderStatus = &runtime.RawExtension{Raw: raw}
			test.EXPECTStatusPatch(ctx, sw, expectedInfra, infra, types.MergePatchType)

			tf.EXPECT().InitializeWith(ctx, gomock.Any()).Return(tf)
			tf.EXPECT().Apply(ctx).Return(errors.New("could not apply terraform config"))
			cloudProviderSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: infra.Namespace}}
			c.EXPECT().Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret).Return(nil)

			err = a.Restore(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#Migrate", func() {
		It("should migrate the Infrastructure", func() {
			tf.EXPECT().EnsureCleanedUp(ctx)
			tf.EXPECT().CleanupConfiguration(ctx)
			tf.EXPECT().RemoveTerraformerFinalizerFromConfig(ctx)
			err := a.Migrate(ctx, log, infra, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if terraformer cannot be created", func() {
			defer test.WithVars(
				&internal.NewTerraformer, func(_ logr.Logger, _ *rest.Config, _ string, _ *extensionsv1alpha1.Infrastructure, _ bool) (terraformer.Terraformer, error) {
					return nil, errors.New("could not create terraform")
				},
			)()
			err := a.Migrate(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if cleaning up terraform configuration fails", func() {
			tf.EXPECT().EnsureCleanedUp(ctx)
			tf.EXPECT().CleanupConfiguration(ctx).Return(errors.New("could not clean up terraform config"))
			err := a.Migrate(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if removal of finalizers on terraform resources fails", func() {
			tf.EXPECT().EnsureCleanedUp(ctx)
			tf.EXPECT().CleanupConfiguration(ctx)
			tf.EXPECT().RemoveTerraformerFinalizerFromConfig(ctx).Return(errors.New("could not clean up finalizers"))
			err := a.Migrate(ctx, log, infra, cluster)
			Expect(err).To(HaveOccurred())
		})
	})
})

func createInfraState(providerStatus *apiv1alpha1.InfrastructureStatus, tfState *terraformer.RawState) (*runtime.RawExtension, error) {
	tfStateData, err := tfState.Marshal()
	if err != nil {
		return nil, err
	}

	infraState := &infrainternal.InfrastructureState{
		SavedProviderStatus: &runtime.RawExtension{
			Object: providerStatus,
		},
		TerraformState: &runtime.RawExtension{
			Raw: tfStateData,
		},
	}

	infraStateData, err := json.Marshal(infraState)
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{Raw: infraStateData}, nil
}

func createInfra(providerConfig *api.InfrastructureConfig) (*extensionsv1alpha1.Infrastructure, error) {
	providerConfigBytes, err := json.Marshal(providerConfig)
	if err != nil {
		return nil, err
	}

	infra := &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           azure.Type,
				ProviderConfig: &runtime.RawExtension{Raw: providerConfigBytes},
			},
			SecretRef: corev1.SecretReference{
				Name: "foo",
			},
			Region: region,
		},
	}

	return infra, nil
}

func createCluster(cloudProfileConfig *api.CloudProfileConfig) (*extensions.Cluster, error) {
	cloudProfileConfigBytes, err := json.Marshal(cloudProfileConfig)
	if err != nil {
		return nil, err
	}

	cluster := &extensions.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Shoot: &v1beta1.Shoot{},
		Seed:  &v1beta1.Seed{},
		CloudProfile: &v1beta1.CloudProfile{
			Spec: v1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: cloudProfileConfigBytes,
				},
			},
		},
	}

	return cluster, nil
}
