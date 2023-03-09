// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infraflow_test

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal/infrastructure"
)

var _ = Describe("TfAdapter", func() {
	location := "westeurope"
	clusterName := "test_cluster"
	infra := &v1alpha1.Infrastructure{Spec: v1alpha1.InfrastructureSpec{Region: location}, ObjectMeta: metav1.ObjectMeta{Namespace: clusterName}}
	cluster := infrastructure.MakeCluster("11.0.0.0/16", "12.0.0.0/16", infra.Spec.Region, 1, 1)
	It("should return the Identity information", func() {
		cfg := newBasicConfig()
		sut, err := infraflow.NewTerraformAdapter(infra, cfg, cluster)
		Expect(err).ToNot(HaveOccurred())
		res := sut.Identity()
		Expect(res).To(BeNil())
	})
	It("should return NAT config for single subnet", func() {
		cfg := newBasicConfig()
		cfg.Networks.NatGateway = &azure.NatGatewayConfig{
			Zone:    to.Ptr(int32(1)),
			Enabled: true,
		}
		sut, err := infraflow.NewTerraformAdapter(infra, cfg, cluster)
		Expect(err).ToNot(HaveOccurred())
		res := sut.Zones()
		Expect(res).NotTo(BeEmpty())
	})
})
