// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"crypto/sha256"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Maximum length for "base" name due to fact that we use this name to name other Azure resources,
// https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules
const maxLengthForBaseName = 33

// Options contains provider-related information required for setting up
// a bastion instance. This struct combines precomputed values like the
// bastion instance name with the IDs of pre-existing cloud provider
// resources, like the nic name etc.
type Options struct {
	BastionInstanceName string
	BastionPublicIPName string
	PrivateIPAddressV4  string
	PrivateIPAddressV6  string
	ResourceGroupName   string
	SecurityGroupName   string
	Location            string
	NicName             string
	NicID               string
	DiskName            string
	SecretReference     corev1.SecretReference
	WorkersCIDR         []string
	CIDRs               []string
	Tags                map[string]*string
	MachineType         string
	ImageRef            *armcompute.ImageReference
}

// DetermineOptions determines the information that are required to reconcile a Bastion on Azure. This
// function does not create any IaaS resources.
func DetermineOptions(bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster, resourceGroup string) (*Options, error) {
	clusterName := cluster.ObjectMeta.Name
	baseResourceName, err := generateBastionBaseResourceName(clusterName, bastion.Name)
	if err != nil {
		return nil, err
	}

	secretReference := corev1.SecretReference{
		Namespace: cluster.ObjectMeta.Name,
		Name:      v1beta1constants.SecretNameCloudProvider,
	}

	cidrs, err := ingressPermissions(bastion)
	if err != nil {
		return nil, err
	}

	workersCidr, err := getWorkersCIDR(cluster)
	if err != nil {
		return nil, err
	}

	tags := map[string]*string{
		"Name": &baseResourceName,
		"Type": to.StringPtr("gardenctl"),
	}

	vmDetails, err := DetermineVmDetails(cluster.CloudProfile.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to determine VM details for bastion host: %w", err)
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cloud provider config from cluster: %w", err)
	}

	imageRef, err := getProviderSpecificImage(cloudProfileConfig.MachineImages, vmDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to extract image from provider config: %w", err)
	}

	return &Options{
		BastionInstanceName: baseResourceName,
		BastionPublicIPName: publicIPResourceName(baseResourceName),
		SecretReference:     secretReference,
		CIDRs:               cidrs,
		WorkersCIDR:         workersCidr,
		DiskName:            DiskResourceName(baseResourceName),
		Location:            cluster.Shoot.Spec.Region,
		ResourceGroupName:   resourceGroup,
		NicName:             NicResourceName(baseResourceName),
		Tags:                tags,
		SecurityGroupName:   NSGName(clusterName),
		MachineType:         vmDetails.MachineName,
		ImageRef:            imageRef,
	}, nil
}

func generateBastionBaseResourceName(clusterName string, bastionName string) (string, error) {
	if clusterName == "" {
		return "", fmt.Errorf("clusterName can't be empty")
	}
	if bastionName == "" {
		return "", fmt.Errorf("bastionName can't be empty")
	}

	staticName := clusterName + "-" + bastionName
	h := sha256.New()
	_, err := h.Write([]byte(staticName))
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))
	if len([]rune(staticName)) > maxLengthForBaseName {
		staticName = staticName[:maxLengthForBaseName]
	}
	return fmt.Sprintf("%s-bastion-%s", staticName, hash[:5]), nil
}

func ingressPermissions(bastion *extensionsv1alpha1.Bastion) ([]string, error) {
	var cidrs []string
	for _, ingress := range bastion.Spec.Ingress {
		cidr := ingress.IPBlock.CIDR
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid ingress CIDR %q: %w", cidr, err)
		}

		normalisedCIDR := ipNet.String()

		if ip.To4() == nil && ip.To16() == nil {
			return nil, fmt.Errorf("ip address %s is not valid: %w", normalisedCIDR, err)
		}

		cidrs = append(cidrs, normalisedCIDR)
	}

	return cidrs, nil
}

// getProviderSpecificImage returns the provider specific MachineImageVersion that matches with the given VmDetails
func getProviderSpecificImage(images []azure.MachineImages, vm VmDetails) (*armcompute.ImageReference, error) {
	imageIndex := slices.IndexFunc(images, func(image azure.MachineImages) bool {
		return image.Name == vm.ImageBaseName
	})

	if imageIndex == -1 {
		return nil, fmt.Errorf("machine image with name %s not found in cloudProfileConfig", vm.ImageBaseName)
	}

	versions := images[imageIndex].Versions
	versionIndex := slices.Index(versions, vm.ImageVersion)

	if versionIndex == -1 {
		return nil, fmt.Errorf("version %s for arch %s of image %s not found in cloudProfileConfig",
			vm.ImageVersion, vm.Architecture, vm.ImageBaseName)
	}

	image := versions[versionIndex]

	var (
		publisher *string
		offer     *string
		sku       *string
	)
	if image.URN != nil {
		urnSplit := strings.Split(*image.URN, ":")
		if len(urnSplit) == 4 {
			publisher = &urnSplit[0]
			offer = &urnSplit[1]
			sku = &urnSplit[2]
		}
	}

	return &armcompute.ImageReference{
		CommunityGalleryImageID: image.CommunityGalleryImageID,
		ID:                      image.ID,
		Publisher:               publisher,
		Offer:                   offer,
		SKU:                     sku,
		SharedGalleryImageID:    image.SharedGalleryImageID,
	}, nil
}

func nodesResourceName(baseName string) string {
	return fmt.Sprintf("%s-nodes", baseName)
}

func publicIPResourceName(baseName string) string {
	return fmt.Sprintf("%s-public-ip", baseName)
}

// NSGIngressAllowSSHResourceNameIPv4 is network security group ingress allow ssh resource name
func NSGIngressAllowSSHResourceNameIPv4(baseName string) string {
	return fmt.Sprintf("%s-allow-ssh-ipv4", baseName)
}

// NSGIngressAllowSSHResourceNameIPv6 is network security group ingress allow ssh resource name
func NSGIngressAllowSSHResourceNameIPv6(baseName string) string {
	return fmt.Sprintf("%s-allow-ssh-ipv6", baseName)
}

// NSGEgressAllowOnlyResourceName is network security group egress allow only rule name
func NSGEgressAllowOnlyResourceName(baseName string) string {
	return fmt.Sprintf("%s-egress-worker", baseName)
}

// NSGEgressDenyAllResourceName is network security group egress deny all rule name
func NSGEgressDenyAllResourceName(baseName string) string {
	return fmt.Sprintf("%s-deny-all", baseName)
}

// NSGName is network security group resource name
func NSGName(baseName string) string {
	return fmt.Sprintf("%s-workers", baseName)
}

// DiskResourceName is Disk resource name
func DiskResourceName(baseName string) string {
	return fmt.Sprintf("%s-disk", baseName)
}

// NicResourceName is Nic resource name
func NicResourceName(baseName string) string {
	return fmt.Sprintf("%s-nic", baseName)
}
