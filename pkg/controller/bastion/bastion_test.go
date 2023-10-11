// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bastion

import (
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	api "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
)

var _ = Describe("Bastion test", func() {
	var (
		cluster *extensions.Cluster
		bastion *extensionsv1alpha1.Bastion

		ctrl                 *gomock.Controller
		maxLengthForResource int
	)
	BeforeEach(func() {
		vNetCIDR := api.VNet{
			CIDR: pointer.String("10.0.0.1/32"),
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
			Expect(len(*rules)).Should(Equal(4))
		})
		It("should return 2 rules without ipv6", func() {
			opt := &Options{
				PrivateIPAddressV4: "1.1.1.1",
				CIDRs:              []string{"213.69.151.131/32", "2001:db8:3333:4444:5555:6666:7777:8888/128"},
			}
			rules := prepareNSGRules(opt)
			Expect(len(*rules)).Should(Equal(3))
		})
	})

	Describe("Testing manipulations with Firewall Rules", func() {
		It("Should be valid", func() {
			ruleSet1 := []network.SecurityRule{
				{Name: pointer.String("ruleName2"), SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
					SourceAddressPrefix:      pointer.String("1.1.1.1"),
					DestinationAddressPrefix: pointer.String("*"),
				}},
				{Name: pointer.String("ruleName1"), SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
					SourceAddressPrefix:      pointer.String("*"),
					DestinationAddressPrefix: pointer.String("*"),
				}},
			}

			ruleSet2 := []network.SecurityRule{
				{Name: pointer.String("ruleName1"), SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
					SourceAddressPrefix:      pointer.String("8.8.8.8"),
					DestinationAddressPrefix: pointer.String("*"),
				}},
			}

			ruleSet3 := []network.SecurityRule{
				{Name: pointer.String("ruleName1"), SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
					SourceAddressPrefix:      pointer.String("*"),
					DestinationAddressPrefix: pointer.String("8.8.8.8"),
				}},
			}

			Expect(expectedNSGRulesPresentAndValid(&ruleSet1, &ruleSet1)).To(Equal(true))
			Expect(expectedNSGRulesPresentAndValid(&ruleSet1, &ruleSet2)).To(Equal(false))
			Expect(expectedNSGRulesPresentAndValid(&ruleSet1, &ruleSet3)).To(Equal(false))
		})

		It("should add Rules with new priority numbers and delete old one", func() {
			oldSet := &[]network.SecurityRule{
				{
					Name:                         pointer.String("defaultRule"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{Priority: pointer.Int32(50)},
				},
				{
					Name:                         pointer.String("ruleName1"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{Priority: pointer.Int32(100)},
				},
				{
					Name:                         pointer.String("ruleName2"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{Priority: pointer.Int32(200)},
				},
			}
			newSet := []network.SecurityRule{
				{
					Name:                         pointer.String("ruleName1"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{Priority: pointer.Int32(100)},
				},
				{
					Name:                         pointer.String("ruleName2"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{Priority: pointer.Int32(200)},
				},
			}

			addOrReplaceNsgRulesDefinition(oldSet, &newSet)
			result := *oldSet

			Expect(*result[0].Name).To(Equal("defaultRule"))
			Expect(*result[1].Name).To(Equal("ruleName1"))
			Expect(*result[2].Name).To(Equal("ruleName2"))

			Expect(*result[0].Priority).To(Equal(int32(50)))
			Expect(*result[1].Priority).To(Equal(int32(101)))
			Expect(*result[2].Priority).To(Equal(int32(201)))
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
			var rulesArr *[]network.SecurityRule = nil
			result := deleteSecurityRuleDefinitionsByName(rulesArr, "")

			Expect(result).To(Equal(false))
			Expect(rulesArr).To(BeNil())
		})

		It("Should return single security rule and keep with NIL name", func() {
			original := &[]network.SecurityRule{
				{Name: nil},
				{Name: pointer.String("ruleName1")},
				{Name: pointer.String("ruleName2")},
				{Name: pointer.String("ruleName3")},
			}

			someDeleted := deleteSecurityRuleDefinitionsByName(original, "ruleName1", "ruleName2", "non-exist-rule")

			Expect(someDeleted).To(Equal(true))
			Expect(original).To(Equal(&[]network.SecurityRule{
				{Name: nil},
				{Name: pointer.String("ruleName3")},
			}))

			Expect(len(*original)).To(Equal(2))
		})
	})

	Describe("check getPrivateIPAddress ", func() {
		nic := &network.Interface{
			InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
				IPConfigurations: &[]network.InterfaceIPConfiguration{
					{
						InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
							PrivateIPAddress: to.StringPtr("192.168.1.2"),
						},
					},
					{
						InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
							PrivateIPAddress: to.StringPtr("2001:db8:3333:4444:5555:6666:7777:8888"),
						},
					},
					{
						InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
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
})

func createShootTestStruct(vNet api.VNet) *gardencorev1beta1.Shoot {
	config := &api.InfrastructureConfig{
		Networks: api.NetworkConfig{
			VNet:             vNet,
			Workers:          pointer.String("10.250.0.0/16"),
			ServiceEndpoints: []string{},
		},
		Zoned: true,
	}

	infrastructureConfigBytes, err := json.Marshal(config)
	Expect(err).NotTo(HaveOccurred())

	return &gardencorev1beta1.Shoot{
		Spec: gardencorev1beta1.ShootSpec{
			Region:            "westeurope",
			SecretBindingName: pointer.String(v1beta1constants.SecretNameCloudProvider),
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
			},
		},
	}
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
