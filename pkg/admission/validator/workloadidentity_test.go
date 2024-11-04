// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/gardener/gardener-extension-provider-azure/pkg/admission/validator"
	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
)

var _ = Describe("WorkloadIdentity validator", func() {
	Describe("#Validate", func() {
		var (
			workloadIdentityValidator extensionswebhook.Validator
			workloadIdentity          *securityv1alpha1.WorkloadIdentity
			ctx                       = context.Background()
		)

		BeforeEach(func() {
			workloadIdentity = &securityv1alpha1.WorkloadIdentity{
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "azure",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
clientID: "11111c4e-db61-17fa-a141-ed39b34aa561"
tenantID: "22222c4e-db61-17fa-a141-ed39b34aa561"
subscriptionID: "33333c4e-db61-17fa-a141-ed39b34aa561"
`),
						},
					},
				},
			}
			scheme := runtime.NewScheme()
			Expect(securityv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(azureapi.AddToScheme(scheme)).To(Succeed())
			Expect(azureapiv1alpha1.AddToScheme(scheme)).To(Succeed())

			workloadIdentityValidator = validator.NewWorkloadIdentityValidator(serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder())
		})

		It("should skip validation if workload identity is not of type 'gcp'", func() {
			workloadIdentity.Spec.TargetSystem.Type = "foo"
			Expect(workloadIdentityValidator.Validate(ctx, workloadIdentity, nil)).To(Succeed())
		})

		It("should successfully validate the creation of a workload identity", func() {
			Expect(workloadIdentityValidator.Validate(ctx, workloadIdentity, nil)).To(Succeed())
		})

		It("should successfully validate the update of a workload identity", func() {
			newWorkloadIdentity := workloadIdentity.DeepCopy()
			newWorkloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
clientID: "44444c4e-db61-17fa-a141-ed39b34aa561"
tenantID: "22222c4e-db61-17fa-a141-ed39b34aa561"
subscriptionID: "33333c4e-db61-17fa-a141-ed39b34aa561"
`)
			Expect(workloadIdentityValidator.Validate(ctx, newWorkloadIdentity, workloadIdentity)).To(Succeed())
		})

		It("should not allow changing the tenantID or subscriptionID", func() {
			newWorkloadIdentity := workloadIdentity.DeepCopy()
			newWorkloadIdentity.Spec.TargetSystem.ProviderConfig.Raw = []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
clientID: "11111c4e-db61-17fa-a141-ed39b34aa561"
tenantID: "44444c4e-db61-17fa-a141-ed39b34aa561"
subscriptionID: "44444c4e-db61-17fa-a141-ed39b34aa561"
`)
			err := workloadIdentityValidator.Validate(ctx, newWorkloadIdentity, workloadIdentity)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.targetSystem.providerConfig.subscriptionID: Invalid value: \"44444c4e-db61-17fa-a141-ed39b34aa561\": field is immutable, spec.targetSystem.providerConfig.tenantID: Invalid value: \"44444c4e-db61-17fa-a141-ed39b34aa561\": field is immutable"))
		})
	})
})
