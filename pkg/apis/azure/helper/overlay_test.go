// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

var _ = Describe("IsOverlayEnabled", func() {
	providerConfig := func(raw string) *gardencorev1beta1.Networking {
		return &gardencorev1beta1.Networking{ProviderConfig: &runtime.RawExtension{Raw: []byte(raw)}}
	}

	It("returns true when networking is nil", func() {
		enabled, err := IsOverlayEnabled(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeTrue())
	})

	It("returns true when no provider config is set and type is unset", func() {
		enabled, err := IsOverlayEnabled(&gardencorev1beta1.Networking{})
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeTrue())
	})

	It("returns false when no provider config is set but type is calico", func() {
		enabled, err := IsOverlayEnabled(&gardencorev1beta1.Networking{Type: ptr.To("calico")})
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeFalse())
	})

	It("returns false for calico when the overlay key is absent from the provider config", func() {
		network := providerConfig(`{"foo":"bar"}`)
		network.Type = ptr.To("calico")
		enabled, err := IsOverlayEnabled(network)
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeFalse())
	})

	It("returns true for cilium when the overlay key is absent from the provider config", func() {
		network := providerConfig(`{"foo":"bar"}`)
		network.Type = ptr.To("cilium")
		enabled, err := IsOverlayEnabled(network)
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeTrue())
	})

	It("returns false for calico when the overlay object has no enabled key", func() {
		network := providerConfig(`{"overlay":{}}`)
		network.Type = ptr.To("calico")
		enabled, err := IsOverlayEnabled(network)
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeFalse())
	})

	It("returns true for cilium when the overlay object has no enabled key", func() {
		network := providerConfig(`{"overlay":{}}`)
		network.Type = ptr.To("cilium")
		enabled, err := IsOverlayEnabled(network)
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeTrue())
	})

	It("honours an explicit overlay.enabled=true regardless of type", func() {
		network := providerConfig(`{"overlay":{"enabled":true}}`)
		network.Type = ptr.To("calico")
		enabled, err := IsOverlayEnabled(network)
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeTrue())
	})

	It("honours an explicit overlay.enabled=false regardless of type", func() {
		network := providerConfig(`{"overlay":{"enabled":false}}`)
		network.Type = ptr.To("cilium")
		enabled, err := IsOverlayEnabled(network)
		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeFalse())
	})

	It("returns an error when overlay.enabled is not a boolean", func() {
		_, err := IsOverlayEnabled(providerConfig(`{"overlay":{"enabled":"yes"}}`))
		Expect(err).To(MatchError("overlay.enabled is not a boolean"))
	})

	It("returns an error when the provider config is not valid JSON", func() {
		_, err := IsOverlayEnabled(providerConfig(`not-json`))
		Expect(err).To(HaveOccurred())
	})
})
