// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-azure/pkg/admission/mutator"
	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/install"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var _ = Describe("NamespacedCloudProfile Mutator", func() {
	var (
		fakeClient  client.Client
		fakeManager manager.Manager
		namespace   string
		ctx         = context.Background()
		decoder     runtime.Decoder

		namespacedCloudProfileMutator extensionswebhook.Mutator
		namespacedCloudProfile        *v1beta1.NamespacedCloudProfile
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(install.AddToScheme(scheme))
		utilruntime.Must(v1beta1.AddToScheme(scheme))
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		fakeManager = &test.FakeManager{
			Client: fakeClient,
			Scheme: scheme,
		}
		namespace = "garden-dev"
		decoder = serializer.NewCodecFactory(fakeManager.GetScheme(), serializer.EnableStrict).UniversalDecoder()

		namespacedCloudProfileMutator = mutator.NewNamespacedCloudProfileMutator(fakeManager)
		namespacedCloudProfile = &v1beta1.NamespacedCloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile-1",
				Namespace: namespace,
			},
		}
	})

	Describe("#Mutate", func() {
		It("should succeed for NamespacedCloudProfile without provider config", func() {
			Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should skip if NamespacedCloudProfile is in deletion phase", func() {
			namespacedCloudProfile.DeletionTimestamp = ptr.To(metav1.Now())
			expectedProfile := namespacedCloudProfile.DeepCopy()

			Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

			Expect(namespacedCloudProfile).To(DeepEqual(expectedProfile))
		})

		Describe("populate spec capabilityFlavors from provider config", func() {
			var parentProfile *v1beta1.CloudProfile

			BeforeEach(func() {
				parentProfile = &v1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{Name: "parent-profile"},
					Spec: v1beta1.CloudProfileSpec{
						MachineCapabilities: []v1beta1.CapabilityDefinition{{
							Name:   "architecture",
							Values: []string{"amd64", "arm64"},
						}},
					},
				}
				namespacedCloudProfile.Spec.Parent = v1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "parent-profile",
				}
			})

			It("should populate capabilityFlavors from old format provider config", func() {
				Expect(fakeClient.Create(ctx, parentProfile)).To(Succeed())

				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"ubuntu","versions":[
    {"version":"22.04","id":"canonical:ubuntu:22.04:latest"},
    {"version":"22.04","architecture":"arm64","id":"canonical:ubuntu:22.04:arm64"}
  ]}
]}`)}
				namespacedCloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
					{Name: "ubuntu", Versions: []v1beta1.MachineImageVersion{
						{ExpirableVersion: v1beta1.ExpirableVersion{Version: "22.04"}},
					}},
				}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				// Should have flavors for both amd64 and arm64. Since acceleratedNetworking
				// is not set (defaults to false), the network capability is set to "basic".
				Expect(namespacedCloudProfile.Spec.MachineImages[0].Versions[0].CapabilityFlavors).To(ConsistOf(
					v1beta1.MachineImageFlavor{Capabilities: v1beta1.Capabilities{
						"architecture":              []string{"amd64"},
						azure.CapabilityNetworkName: []string{azure.CapabilityNetworkBasic},
					}},
					v1beta1.MachineImageFlavor{Capabilities: v1beta1.Capabilities{
						"architecture":              []string{"arm64"},
						azure.CapabilityNetworkName: []string{azure.CapabilityNetworkBasic},
					}},
				))
			})

			It("should populate capabilityFlavors from old format provider config with acceleratedNetworking", func() {
				Expect(fakeClient.Create(ctx, parentProfile)).To(Succeed())

				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"ubuntu","versions":[
    {"version":"22.04","acceleratedNetworking":true,"id":"canonical:ubuntu:22.04:latest"},
    {"version":"22.04","acceleratedNetworking":false,"id":"canonical:ubuntu:22.04:basic"}
  ]}
]}`)}
				namespacedCloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
					{Name: "ubuntu", Versions: []v1beta1.MachineImageVersion{
						{ExpirableVersion: v1beta1.ExpirableVersion{Version: "22.04"}},
					}},
				}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				// When acceleratedNetworking=true the network capability is omitted from the flavor
				// (meaning any networking is acceptable). When acceleratedNetworking=false the
				// network capability is restricted to "basic".
				Expect(namespacedCloudProfile.Spec.MachineImages[0].Versions[0].CapabilityFlavors).To(ConsistOf(
					v1beta1.MachineImageFlavor{Capabilities: v1beta1.Capabilities{
						"architecture": []string{"amd64"},
					}},
					v1beta1.MachineImageFlavor{Capabilities: v1beta1.Capabilities{
						"architecture":              []string{"amd64"},
						azure.CapabilityNetworkName: []string{azure.CapabilityNetworkBasic},
					}},
				))
			})

			It("should populate capabilityFlavors from new format provider config", func() {
				Expect(fakeClient.Create(ctx, parentProfile)).To(Succeed())

				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"ubuntu","versions":[{"version":"22.04","capabilityFlavors":[
    {"capabilities":{"architecture":["amd64"]},"id":"canonical:ubuntu:22.04:latest"},
    {"capabilities":{"architecture":["arm64"]},"id":"canonical:ubuntu:22.04:arm64"}
  ]}]}
]}`)}
				namespacedCloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
					{Name: "ubuntu", Versions: []v1beta1.MachineImageVersion{
						{ExpirableVersion: v1beta1.ExpirableVersion{Version: "22.04"}},
					}},
				}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				Expect(namespacedCloudProfile.Spec.MachineImages[0].Versions[0].CapabilityFlavors).To(ConsistOf(
					v1beta1.MachineImageFlavor{Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}}},
					v1beta1.MachineImageFlavor{Capabilities: v1beta1.Capabilities{"architecture": []string{"arm64"}}},
				))
			})

			It("should skip spec mutation when parent has no machineCapabilities", func() {
				parentProfile.Spec.MachineCapabilities = nil
				Expect(fakeClient.Create(ctx, parentProfile)).To(Succeed())

				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"ubuntu","versions":[{"version":"22.04","id":"canonical:ubuntu:22.04:latest"}]}
]}`)}
				namespacedCloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
					{Name: "ubuntu", Versions: []v1beta1.MachineImageVersion{
						{ExpirableVersion: v1beta1.ExpirableVersion{Version: "22.04"}},
					}},
				}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				// capabilityFlavors should NOT be populated
				Expect(namespacedCloudProfile.Spec.MachineImages[0].Versions[0].CapabilityFlavors).To(BeEmpty())
			})

			It("should skip spec mutation when Spec.Parent.Name is empty", func() {
				namespacedCloudProfile.Spec.Parent = v1beta1.CloudProfileReference{}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"ubuntu","versions":[{"version":"22.04","id":"canonical:ubuntu:22.04:latest"}]}
]}`)}
				namespacedCloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
					{Name: "ubuntu", Versions: []v1beta1.MachineImageVersion{
						{ExpirableVersion: v1beta1.ExpirableVersion{Version: "22.04"}},
					}},
				}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				// capabilityFlavors should NOT be populated
				Expect(namespacedCloudProfile.Spec.MachineImages[0].Versions[0].CapabilityFlavors).To(BeEmpty())
			})

			It("should skip spec mutation when Spec.MachineImages is empty", func() {
				Expect(fakeClient.Create(ctx, parentProfile)).To(Succeed())

				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"ubuntu","versions":[{"version":"22.04","id":"canonical:ubuntu:22.04:latest"}]}
]}`)}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())
			})
		})

		Describe("merge the provider configurations from a NamespacedCloudProfile and the parent CloudProfile", func() {
			It("should correctly merge extended machineImages", func() {
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[
		{"version":"1.0","id":"local/image:1.0"},
		{"version":"1.0","architecture": "arm64", "id":"local/image:1.0"}
]}
]}`)}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.1","id":"local/image:1.1"}]},
  {"name":"image-2","versions":[{"version":"2.0","id":"local/image:2.0"}]}
]}`)}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				mergedConfig, err := decodeCloudProfileConfig(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(mergedConfig.MachineImages).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-1"),
						"Versions": ContainElements(
							api.MachineImageVersion{Version: "1.0", Image: api.Image{ID: ptr.To("local/image:1.0")}, Architecture: ptr.To("amd64")},
							api.MachineImageVersion{Version: "1.0", Image: api.Image{ID: ptr.To("local/image:1.0")}, Architecture: ptr.To("arm64")},
							api.MachineImageVersion{Version: "1.1", Image: api.Image{ID: ptr.To("local/image:1.1")}, Architecture: ptr.To("amd64")},
						),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Name":     Equal("image-2"),
						"Versions": ContainElements(api.MachineImageVersion{Version: "2.0", Image: api.Image{ID: ptr.To("local/image:2.0")}, Architecture: ptr.To("amd64")}),
					}),
				))
			})
			It("should correctly merge extended machineImages using capabilities ", func() {
				namespacedCloudProfile.Status.CloudProfileSpec.MachineCapabilities = []v1beta1.CapabilityDefinition{{
					Name:   "architecture",
					Values: []string{"amd64", "arm64"},
				}}
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","capabilityFlavors":[
{"capabilities":{"architecture":["amd64"]},"id":"local/image:1.0"}
]}]}
]}`)}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.1","capabilityFlavors":[
{"capabilities":{"architecture":["amd64"]},"id":"local/image:1.1"},
{"capabilities":{"architecture":["arm64"]},"id":"local/image:1.1"}
]}]},
  {"name":"image-2","versions":[{"version":"2.0","capabilityFlavors":[
{"capabilities":{"architecture":["amd64"]},"id":"local/image:2.0"}
]}]}
]}`)}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				mergedConfig, err := decodeCloudProfileConfig(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(mergedConfig.MachineImages).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-1"),
						"Versions": ContainElements(
							api.MachineImageVersion{Version: "1.0",
								CapabilityFlavors: []api.MachineImageFlavor{{
									Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
									Image:        api.Image{ID: ptr.To("local/image:1.0")}}},
							},
							api.MachineImageVersion{Version: "1.1",
								CapabilityFlavors: []api.MachineImageFlavor{
									{
										Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
										Image:        api.Image{ID: ptr.To("local/image:1.1")},
									},
									{
										Capabilities: v1beta1.Capabilities{"architecture": []string{"arm64"}},
										Image:        api.Image{ID: ptr.To("local/image:1.1")},
									},
								},
							},
						),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-2"),
						"Versions": ContainElements(
							api.MachineImageVersion{Version: "2.0",
								CapabilityFlavors: []api.MachineImageFlavor{{
									Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}},
									Image:        api.Image{ID: ptr.To("local/image:2.0")}}},
							}),
					}),
				))
			})

			It("should correctly merge added machineTypes", func() {
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineTypes":[{"name":"type-1"}]}`)}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"azure.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineTypes":[{"name":"type-2","acceleratedNetworking":true}]}`)}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				mergedConfig, err := decodeCloudProfileConfig(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig)
				Expect(err).ToNot(HaveOccurred())
				var boolNil *bool
				Expect(mergedConfig.MachineTypes).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{"Name": Equal("type-1"), "AcceleratedNetworking": BeEquivalentTo(boolNil)}),
					MatchFields(IgnoreExtras, Fields{"Name": Equal("type-2"), "AcceleratedNetworking": BeEquivalentTo(ptr.To(true))}),
				))
			})
		})
	})
})

func decodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*api.CloudProfileConfig, error) {
	cloudProfileConfig := &api.CloudProfileConfig{}
	if err := util.Decode(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}
	return cloudProfileConfig, nil
}
