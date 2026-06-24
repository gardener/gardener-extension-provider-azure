// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider_test

import (
	"context"
	"testing"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
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

		secret                 *corev1.Secret
		servicePrincipalSecret corev1.Secret

		gctx = gcontext.NewGardenContext(nil, nil)
	)

	// purposeLabel is the label used to identify tenant service principal secrets.
	const purposeLabel = "azure.provider.extensions.gardener.cloud/purpose"
	const purposeValue = "tenant-service-principal-secret"

	newEnsurer := func(objs ...corev1.Secret) cloudprovider.Ensurer {
		scheme := kubernetes.SeedScheme
		Expect(install.AddToScheme(scheme)).To(Succeed())
		builder := fakeclient.NewClientBuilder().WithScheme(scheme)
		for i := range objs {
			builder = builder.WithObjects(&objs[i])
		}
		mgr := testutils.FakeManager{Client: builder.Build()}
		return NewEnsurer(mgr, logger)
	}

	BeforeEach(func() {
		secret = &corev1.Secret{
			Data: map[string][]byte{
				"tenantID": []byte("tenant-id"),
			},
		}
		servicePrincipalSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc-principal",
				Namespace: "default",
				Labels: map[string]string{
					purposeLabel: purposeValue,
				},
			},
			Data: map[string][]byte{
				"tenantID":     []byte("tenant-id"),
				"clientID":     []byte("client-id"),
				"clientSecret": []byte("client-secret"),
			},
		}

		ensurer = newEnsurer()
	})

	Describe("#EnsureCloudProviderSecret", func() {
		It("should pass as clientID and clientSecret are present", func() {
			secret.Data["clientID"] = []byte("client-id")
			secret.Data["clientSecret"] = []byte("client-secret")

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail as no tenantID is present", func() {
			delete(secret.Data, "tenantID")
			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should add clientID and clientSecret", func() {
			ensurer = newEnsurer(servicePrincipalSecret)

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data).To(Equal(map[string][]byte{
				"tenantID":     []byte("tenant-id"),
				"clientID":     []byte("client-id"),
				"clientSecret": []byte("client-secret"),
			}))
		})

		It("should fail as service principal secret matching to the tenant id exists", func() {
			// No service principal secret pre-populated → list returns empty

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as multiple service principal secrets matching to the tenant id exists", func() {
			sps2 := servicePrincipalSecret.DeepCopy()
			sps2.Name = "svc-principal-2"
			ensurer = newEnsurer(servicePrincipalSecret, *sps2)

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should fail as multiple service principal secrets matching to the tenant id exists", func() {
			servicePrincipalSecret.Data["tenantID"] = []byte("some-different-tenant-id")
			ensurer = newEnsurer(servicePrincipalSecret)

			err := ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should not add workload identity config to the secret if it is not labeled correctly", func() {
			ensurer = newEnsurer(servicePrincipalSecret)
			secret.Labels = map[string]string{"workloadidentity.security.gardener.cloud/provider": "foo"}
			expected := secret.DeepCopy()
			Expect(ensurer.EnsureCloudProviderSecret(ctx, gctx, secret, nil)).To(Succeed())
			expected.Data = map[string][]byte{
				"tenantID":     []byte("tenant-id"),
				"clientID":     []byte("client-id"),
				"clientSecret": []byte("client-secret"),
			}
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
