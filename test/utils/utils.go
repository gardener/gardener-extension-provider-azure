// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	apiv1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
)

// MakeCluster returns a cluster object used for testing.
func MakeCluster(pods, services string, region string, countFaultDomain, countUpdateDomain int32) *controller.Cluster {
	var (
		shoot = gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					Pods:     &pods,
					Services: &services,
				},
			},
		}
		cloudProfileConfig = apiv1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
			CountFaultDomains: []apiv1alpha1.DomainCount{
				{Region: region, Count: countFaultDomain},
			},
			CountUpdateDomains: []apiv1alpha1.DomainCount{
				{Region: region, Count: countUpdateDomain},
			},
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)
		cloudProfile              = gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: cloudProfileConfigJSON,
				},
			},
		}
	)

	return &controller.Cluster{
		Shoot:        &shoot,
		CloudProfile: &cloudProfile,
	}
}

type eqMatcher struct {
	want interface{}
}

// EqMatcher is an equality matcher with custom error format.
func EqMatcher(want interface{}) eqMatcher {
	return eqMatcher{
		want: want,
	}
}

// Matches returns true if argument matches.
func (eq eqMatcher) Matches(got interface{}) bool {
	return gomock.Eq(eq.want).Matches(got)
}

// Got returns the value that was compared to.
func (eq eqMatcher) Got(got interface{}) string {
	return fmt.Sprintf("%v (%T)\nDiff (-got +want):\n%s", got, got, strings.TrimSpace(cmp.Diff(got, eq.want)))
}

// String returns the value that was expected.
func (eq eqMatcher) String() string {
	return fmt.Sprintf("%v (%T)\n", eq.want, eq.want)
}
