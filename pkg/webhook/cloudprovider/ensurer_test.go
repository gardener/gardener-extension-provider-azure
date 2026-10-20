// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider_test

import (
	"context"
	"testing"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/webhook/cloudprovider"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		logger  = log.Log.WithName("azure-cloudprovider-webhook-test")
		ctx     = context.TODO()
		ensurer cloudprovider.Ensurer

		secret *corev1.Secret

		gctx = gcontext.NewGardenContext(nil, nil)
	)

	newEnsurer := func() cloudprovider.Ensurer {
		mgr := testutils.FakeManager{Client: fakeclient.NewClientBuilder().Build()}
		return NewEnsurer(mgr, logger)
	}

	BeforeEach(func() {
		secret = &corev1.Secret{
			Data: map[string][]byte{
				"tenantID":     []byte("tenant-id"),
				"clientID":     []byte("client-id"),
				"clientSecret": []byte("client-secret"),
			},
		}

		ensurer = newEnsurer()
	})

	Describe("#EnsureCloudProviderSecret", func() {
		It("should pass as tenantID, clientID and clientSecret are present", func() {
			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail as no tenantID is present", func() {
			delete(secret.Data, "tenantID")
			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as clientID is missing", func() {
			delete(secret.Data, "clientID")
			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as clientSecret is missing", func() {
			delete(secret.Data, "clientSecret")
			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should not add workload identity config to the secret if it is not labeled correctly", func() {
			secret.Labels = map[string]string{"workloadidentity.security.gardener.cloud/provider": "foo"}
			expected := secret.DeepCopy()
			Expect(ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)).To(Succeed())
			Expect(secret).To(Equal(expected))
		})

		It("should error if cloudprovider secret does not contain config data key but is labeled correctly", func() {
			secret.Labels = map[string]string{"workloadidentity.security.gardener.cloud/provider": "azure"}
			err := ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)
			Expect(err).To(HaveOccurred())

			Expect(err).To(MatchError("cloudprovider secret is missing a 'config' data key"))
		})

		It("should error if cloudprovider secret does not contain a valid WorkloadIdentityConfig", func() {
			secret.Data["config"] = []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfigInvalid
`)
			secret.Labels = map[string]string{"workloadidentity.security.gardener.cloud/provider": "azure"}
			err := ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)
			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("could not decode 'config' as WorkloadIdentityConfig"))
		})

		It("should add config to cloudprovider secret with if it contains WorkloadIdentityConfig", func() {
			secret.Data = map[string][]byte{
				"config": []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
clientID: "client"
tenantID: "tenant"
subscriptionID: "subscription"
`)}
			secret.Labels = map[string]string{"workloadidentity.security.gardener.cloud/provider": "azure"}
			Expect(ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)).To(Succeed())
			Expect(secret.Data).To(Equal(map[string][]byte{
				"config": []byte(`
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
clientID: "client"
tenantID: "tenant"
subscriptionID: "subscription"
`),
				"clientID":                  []byte("client"),
				"tenantID":                  []byte("tenant"),
				"subscriptionID":            []byte("subscription"),
				"workloadIdentityTokenFile": []byte("/var/run/secrets/gardener.cloud/workload-identity/token"),
			}))
		})
	})
})
