// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

func createRule(name, sourceAddrPrefix, destinationAddressPrefix string) *armnetwork.SecurityRule {
	return &armnetwork.SecurityRule{
		Name: ptr.To(name), Properties: &armnetwork.SecurityRulePropertiesFormat{
			SourceAddressPrefix:      ptr.To(sourceAddrPrefix),
			DestinationAddressPrefix: ptr.To(destinationAddressPrefix),
		},
	}
}

func createRuleWithPriority(name string, priority int32) *armnetwork.SecurityRule {
	return &armnetwork.SecurityRule{
		Name: ptr.To(name), Properties: &armnetwork.SecurityRulePropertiesFormat{
			Priority: to.Int32Ptr(priority),
		},
	}
}

var _ = Describe("Bastion test", func() {
	var (
		cluster *extensions.Cluster
		bastion *extensionsv1alpha1.Bastion

		ctrl                 *gomock.Controller
		maxLengthForResource int
		providerImages       []api.MachineImages
	)
	BeforeEach(func() {
		vNetCIDR := api.VNet{
			CIDR: ptr.To("10.0.0.1/32"),
		}
		cluster = createAzureTestCluster(vNetCIDR)
		bastion = createTestBastion()
		ctrl = gomock.NewController(GinkgoT())
		maxLengthForResource = 63
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Testing prepared set of rules based on provided CIDRs", func() {
		It("should return 4 rules", func() {
			opt := &Options{
				PrivateIPAddressV4: "1.1.1.1",
				PrivateIPAddressV6: "::2.2.2.2",
				CIDRs:              []string{"213.69.151.131/32", "2001:db8:3333:4444:5555:6666:7777:8888/128"},
			}
			rules := prepareNSGRules(opt)
			Expect(len(rules)).Should(Equal(4))
		})
		It("should return 2 rules without ipv6", func() {
			opt := &Options{
				PrivateIPAddressV4: "1.1.1.1",
				CIDRs:              []string{"213.69.151.131/32", "2001:db8:3333:4444:5555:6666:7777:8888/128"},
			}
			rules := prepareNSGRules(opt)
			Expect(len(rules)).Should(Equal(3))
		})
	})

	Describe("Testing manipulations with Firewall Rules", func() {
		It("Should be valid", func() {
			ruleSet1 := []*armnetwork.SecurityRule{
				createRule("ruleName2", "1.1.1.1", "*"),
				createRule("ruleName1", "*", "*"),
			}

			ruleSet2 := []*armnetwork.SecurityRule{
				createRule("ruleName1", "8.8.8.8", "*"),
			}

			ruleSet3 := []*armnetwork.SecurityRule{
				createRule("ruleName1", "*", "8.8.8.8"),
			}

			Expect(expectedNSGRulesPresentAndValid(ruleSet1, ruleSet1)).To(Equal(true))
			Expect(expectedNSGRulesPresentAndValid(ruleSet1, ruleSet2)).To(Equal(false))
			Expect(expectedNSGRulesPresentAndValid(ruleSet1, ruleSet3)).To(Equal(false))
		})

		It("should add Rules with new priority numbers and delete old one", func() {
			oldSet := []*armnetwork.SecurityRule{
				createRuleWithPriority("defaultRule", 50),
				createRuleWithPriority("ruleName1", 100),
				createRuleWithPriority("ruleName2", 200),
			}
			newSet := []*armnetwork.SecurityRule{
				createRuleWithPriority("ruleName1", 100),
				createRuleWithPriority("ruleName2", 200),
			}

			result := addOrReplaceNsgRulesDefinition(oldSet, newSet)

			Expect(*result[0].Name).To(Equal("defaultRule"))
			Expect(*result[1].Name).To(Equal("ruleName1"))
			Expect(*result[2].Name).To(Equal("ruleName2"))

			Expect(*result[0].Properties.Priority).To(Equal(int32(50)))
			Expect(*result[1].Properties.Priority).To(Equal(int32(101)))
			Expect(*result[2].Properties.Priority).To(Equal(int32(201)))
		})
	})

	It("should return next free number", func() {
		set := make(map[int32]bool)
		set[10] = true
		set[11] = true

		Expect(*findNextFreeNumber(set, 10)).To(Equal(int32(12)))
		Expect(len(set)).To(Equal(3))
	})

	Describe("getWorkersCIDR", func() {
		It("getWorkersCIDR", func() {
			cidr, err := getWorkersCIDR(cluster)
			Expect(err).To(Not(HaveOccurred()))
			Expect(cidr).To(Equal([]string{"10.250.0.0/16"}))
		})
	})

	Describe("Determine options", func() {
		It("should return options", func() {
			options, err := DetermineOptions(bastion, cluster, "cluster1")
			Expect(err).To(Not(HaveOccurred()))

			Expect(options.BastionInstanceName).To(Equal("cluster1-bastionName1-bastion-1cdc8"))
			Expect(options.BastionPublicIPName).To(Equal("cluster1-bastionName1-bastion-1cdc8-public-ip"))
			Expect(options.SecretReference).To(Equal(corev1.SecretReference{
				Namespace: "cluster1",
				Name:      "cloudprovider",
			}))
			Expect(options.CIDRs).To(Equal([]string{"213.69.151.0/24"}))
			Expect(options.WorkersCIDR).To(Equal([]string{"10.250.0.0/16"}))
			Expect(options.DiskName).To(Equal("cluster1-bastionName1-bastion-1cdc8-disk"))
			Expect(options.Location).To(Equal("westeurope"))
			Expect(options.ResourceGroupName).To(Equal("cluster1"))
			Expect(options.NicName).To(Equal("cluster1-bastionName1-bastion-1cdc8-nic"))
			Expect(options.Tags).To(Equal(map[string]*string{
				"Name": to.StringPtr("cluster1-bastionName1-bastion-1cdc8"),
				"Type": to.StringPtr("gardenctl"),
			}))
			Expect(options.SecurityGroupName).To(Equal("cluster1-workers"))
			Expect(options.MachineType).To(Equal("machineName"))
			Expect(*options.ImageRef.CommunityGalleryImageID).To(Equal("/CommunityGalleries/gardenlinux-1.2.3"))
		})
	})

	Describe("check Names generations", func() {
		It("should generate idempotent name", func() {
			expected := "clusterName-shortName-bastion-79641"

			res, err := generateBastionBaseResourceName("clusterName", "shortName")
			Expect(err).To(Not(HaveOccurred()))
			Expect(res).To(Equal(expected))

			res, err = generateBastionBaseResourceName("clusterName", "shortName")
			Expect(err).To(Not(HaveOccurred()))
			Expect(res).To(Equal(expected))
		})

		It("should generate a name not exceeding a certain length", func() {
			res, err := generateBastionBaseResourceName("clusterName", "LetsExceed63LenLimit012345678901234567890123456789012345678901234567890123456789")
			Expect(err).To(Not(HaveOccurred()))
			Expect(res).To(Equal("clusterName-LetsExceed63LenLimit0-bastion-139c4"))
		})

		It("should generate a unique name even if inputs values have minor deviations", func() {
			res, _ := generateBastionBaseResourceName("1", "1")
			res2, _ := generateBastionBaseResourceName("1", "2")
			Expect(res).ToNot(Equal(res2))
		})

		baseName, _ := generateBastionBaseResourceName("clusterName", "LetsExceed63LenLimit012345678901234567890123456789012345678901234567890123456789")
		DescribeTable("should generate names and fit maximum length",
			func(input string, expectedOut string) {
				Expect(len(input)).Should(BeNumerically("<", maxLengthForResource))
				Expect(input).Should(Equal(expectedOut))
			},

			Entry("nodes resource name", nodesResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-nodes"),
			Entry("public resource name", publicIPResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-public-ip"),
			Entry("nsg ingress ssh resource name", NSGIngressAllowSSHResourceNameIPv4(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-allow-ssh-ipv4"),
			Entry("nsg ingress ssh resource name", NSGIngressAllowSSHResourceNameIPv6(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-allow-ssh-ipv6"),
			Entry("nsg egress allow resource name", NSGEgressAllowOnlyResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-egress-worker"),
			Entry("nsg egress deny resource name", NSGEgressDenyAllResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-deny-all"),
			Entry("nsg name", NSGName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-workers"),
			Entry("disk resource name", DiskResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-disk"),
			Entry("nic resource name", NicResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-nic"),
		)
	})

	Describe("check Ingress Permissions", func() {
		It("Should return a string array with ipV4 normalized addresses", func() {
			bastion.Spec.Ingress = []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "213.69.151.253/24",
				}},
			}
			res, err := ingressPermissions(bastion)
			Expect(err).To(Not(HaveOccurred()))
			Expect(res[0]).To(Equal("213.69.151.0/24"))
		})
		It("Should throw an error with invalid CIDR entry", func() {
			bastion.Spec.Ingress = []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "1234",
				}},
			}
			res, err := ingressPermissions(bastion)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeEmpty())
		})
	})
	Describe("check Ingress Permissions for IPv6", func() {
		It("Should return a string array with IPv6 normalized addresses", func() {
			bastion.Spec.Ingress = []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "2001:1db8:85a3:1111:1111:8a2e:1370:7334/128",
				}},
			}
			res, err := ingressPermissions(bastion)
			Expect(err).To(Not(HaveOccurred()))
			Expect(res[0]).To(Equal("2001:1db8:85a3:1111:1111:8a2e:1370:7334/128"))
		})
	})

	Describe("remove Security Rule", func() {
		It("should return nil if pass nil", func() {
			var rulesArr []*armnetwork.SecurityRule = nil
			var deleted bool
			rulesArr, deleted = deleteSecurityRuleDefinitionsByName(rulesArr, "")

			Expect(deleted).To(Equal(false))
			Expect(rulesArr).To(BeNil())
		})

		It("Should return single security rule and keep with NIL name", func() {
			original := []*armnetwork.SecurityRule{
				{Name: nil},
				{Name: ptr.To("ruleName1")},
				{Name: ptr.To("ruleName2")},
				{Name: ptr.To("ruleName3")},
			}

			modified, someDeleted := deleteSecurityRuleDefinitionsByName(original, "ruleName1", "ruleName2", "non-exist-rule")

			Expect(someDeleted).To(Equal(true))
			Expect(modified).To(Equal([]*armnetwork.SecurityRule{
				{Name: nil},
				{Name: ptr.To("ruleName3")},
			}))

			Expect(len(modified)).To(Equal(2))
		})
	})

	Describe("check getPrivateIPAddress ", func() {
		nic := &armnetwork.Interface{
			Properties: &armnetwork.InterfacePropertiesFormat{
				IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
					{
						Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
							PrivateIPAddress: to.StringPtr("192.168.1.2"),
						},
					},
					{
						Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
							PrivateIPAddress: to.StringPtr("2001:db8:3333:4444:5555:6666:7777:8888"),
						},
					},
					{
						Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
							PrivateIPAddress: to.StringPtr("192.168.1.1"),
						},
					},
				},
			},
		}

		It("should return Private IPv6 Address", func() {
			ip, err := getPrivateIPv6Address(nic)
			Expect(ip).To(Equal("2001:db8:3333:4444:5555:6666:7777:8888"))
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should return Private IPv4 Address", func() {
			ip, err := getPrivateIPv4Address(nic)
			Expect(ip).To(Equal("192.168.1.2"))
			Expect(err).To(Not(HaveOccurred()))
		})
	})

	Describe("#getProviderSpecificImage", func() {
		var desiredVM = VmDetails{
			MachineName:   "machineName",
			Architecture:  "amd64",
			ImageBaseName: "gardenlinux",
			ImageVersion:  "1.2.3",
		}

		It("should succeed for existing communityGallery image", func() {
			providerImages = createTestProviderConfig(armcompute.ImageReference{
				CommunityGalleryImageID: ptr.To("CommunityGalleries/gardenlinux-1.2.3"),
			}).MachineImages
			machineImage, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).ToNot(HaveOccurred())
			Expect(machineImage.CommunityGalleryImageID).To(Equal("CommunityGalleries/gardenlinux-1.2.3"))
		})

		// TODO
		//It("should succeed for existing urn", func() {
		//	if imageRef.Offer != nil && imageRef.Publisher != nil && imageRef.SKU != nil && imageRef.Version != nil {
		//	providerImages = createTestProviderConfig(armcompute.ImageReference{
		//		Offer: ptr.To(),
		//	}).MachineImages
		//	machineImage, err := getProviderSpecificImage(providerImages, desiredVM)
		//	Expect(err).ToNot(HaveOccurred())
		//	Expect(machineImage.CommunityGalleryImageID).To(Equal("CommunityGalleries/gardenlinux-1.2.3"))
		//})

		It("fail if image name does not exist", func() {
			desiredVM.ImageBaseName = "unknown"
			_, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).To(HaveOccurred())
		})

		It("fail if image version does not exist", func() {
			desiredVM.ImageVersion = "6.6.6"
			_, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).To(HaveOccurred())
		})
	})
})

func createShootTestStruct(vNet api.VNet) *gardencorev1beta1.Shoot {
	config := &api.InfrastructureConfig{
		Networks: api.NetworkConfig{
			VNet:             vNet,
			Workers:          ptr.To("10.250.0.0/16"),
			ServiceEndpoints: []string{},
		},
		Zoned: true,
	}

	infrastructureConfigBytes, err := json.Marshal(config)
	Expect(err).NotTo(HaveOccurred())

	return &gardencorev1beta1.Shoot{
		Spec: gardencorev1beta1.ShootSpec{
			Region:            "westeurope",
			SecretBindingName: ptr.To(v1beta1constants.SecretNameCloudProvider),
			Provider: gardencorev1beta1.Provider{
				InfrastructureConfig: &runtime.RawExtension{
					Raw: infrastructureConfigBytes,
				},
			},
		},
	}
}

func createAzureTestCluster(vNet api.VNet) *extensions.Cluster {
	return &controller.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
		Shoot:      createShootTestStruct(vNet),
		CloudProfile: &gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				Regions: []gardencorev1beta1.Region{
					{Name: "westeurope"},
				},
				MachineImages: createTestMachineImages(),
				MachineTypes:  createTestMachineTypes(),
				ProviderConfig: &runtime.RawExtension{
					Raw: mustEncode(createTestProviderConfig(armcompute.ImageReference{
						CommunityGalleryImageID: ptr.To("/CommunityGalleries/gardenlinux-1.2.3"),
					})),
				},
			},
		},
	}
}

func createTestMachineImages() []gardencorev1beta1.MachineImage {
	return []gardencorev1beta1.MachineImage{{
		Name: "gardenlinux",
		Versions: []gardencorev1beta1.MachineImageVersion{{
			ExpirableVersion: gardencorev1beta1.ExpirableVersion{
				Version:        "1.2.3",
				Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
			},
			Architectures: []string{"amd64"},
		}},
	}}
}

func createTestProviderConfig(imageRef armcompute.ImageReference) *api.CloudProfileConfig {
	var urn *string
	if imageRef.Offer != nil && imageRef.Publisher != nil && imageRef.SKU != nil && imageRef.Version != nil {
		urn = ptr.To(fmt.Sprintf("%s:%s:%s:%s, *imageRef.Offer, *imageRef.Publisher, *imageRef.SKU, *imageRef.Version))
	}
	return &api.CloudProfileConfig{MachineImages: []api.MachineImages{{
		Name: "gardenlinux",
		Versions: []api.MachineImageVersion{{
			Version:                 "1.2.3",
			CommunityGalleryImageID: imageRef.CommunityGalleryImageID,
			SharedGalleryImageID:    imageRef.SharedGalleryImageID,
			ID:                      imageRef.ID,
			URN:                     urn,
		}},
	}}}
}

func createTestMachineTypes() []gardencorev1beta1.MachineType {
	return []gardencorev1beta1.MachineType{{
		CPU:          resource.MustParse("4"),
		Name:         "machineName",
		Architecture: ptr.To("amd64"),
	}}
}

func createTestBastion() *extensionsv1alpha1.Bastion {
	return &extensionsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bastionName1",
		},
		Spec: extensionsv1alpha1.BastionSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{},
			UserData:    nil,
			Ingress: []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "213.69.151.0/24",
				}},
			},
		},
	}
}

func mustEncode(object any) []byte {
	data, err := json.Marshal(object)
	Expect(err).To(Not(HaveOccurred()))
	return data
}
