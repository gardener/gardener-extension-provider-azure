// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// daemonSetName is the name of the calico daemon set to mutate.
	daemonSetName = "calico-node"
	// containerName is the name of the calico container to mutate.
	containerName = daemonSetName
	// bpfEnvVariableName is the name of the environment variable indicating whether calico's ebpf dataplane is active or not.
	bpfEnvVariableName = "FELIX_BPFENABLED"
	// bpfDataIfacePatternEnvVariableName is the name of the environment variable to be set during the mutation.
	bpfDataIfacePatternEnvVariableName = "FELIX_BPFDATAIFACEPATTERN"
)

// bpfDataIfacePatternValue is the value to be added as environment variable FELIX_BPFDATAIFACEPATTERN in calico-node.
// Value used in calico v3.26.1: ^((en|wl|ww|sl|ib)[Popsx].*|(eth|wlan|wwan).*|tunl0$|vxlan.calico$|wireguard.cali$|wg-v6.cali$)
// (see https://github.com/projectcalico/calico/blob/v3.26.1/felix/config/config_params.go#L179)
// Removing 'P' ensures that enP... devices used by accelerated networking in azure are not matched.
var bpfDataIfacePatternValue = "^((en|wl|ww|sl|ib)[opsx].*|(eth|wlan|wwan).*|tunl0$|vxlan.calico$|wireguard.cali$|wg-v6.cali$)"

// NewMutator creates a new accelerated network mutator.
func NewMutator(mgr manager.Manager, logger logr.Logger) webhook.Mutator {
	return &mutator{
		client: mgr.GetClient(),
		logger: logger.WithName("mutator"),
	}
}

type mutator struct {
	client client.Client
	logger logr.Logger
}

// Mutate validates and if needed mutates the given object.
func (m *mutator) Mutate(_ context.Context, new, old client.Object) error {
	var (
		newDaemonSet, oldDaemonSet *appsv1.DaemonSet
		ok                         bool
	)

	// If the object does have a deletion timestamp then we don't want to mutate anything.
	if new.GetDeletionTimestamp() != nil {
		return nil
	}

	newDaemonSet, ok = new.(*appsv1.DaemonSet)
	if !ok {
		return fmt.Errorf("could not mutate, object is not of type \"DaemonSet\"")
	}

	if old != nil {
		oldDaemonSet, ok = old.(*appsv1.DaemonSet)
		if !ok {
			return fmt.Errorf("could not cast old object to appsv1.DaemonSet")
		}
	}

	// Only mutate calico-node daemon set in kube-system namespace
	if newDaemonSet.Namespace != metav1.NamespaceSystem || newDaemonSet.Name != daemonSetName {
		return nil
	}

	// Only mutate if calico-node's ebpf mode is active
	ebpfEnabled := false
outer:
	for _, container := range newDaemonSet.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			for _, variable := range container.Env {
				if variable.Name == bpfEnvVariableName {
					ebpfEnabled, _ = strconv.ParseBool(variable.Value)
					break outer
				}
			}
		}
	}
	if !ebpfEnabled {
		return nil
	}

	return m.mutateDaemonSet(newDaemonSet, oldDaemonSet)
}

func (m *mutator) mutateDaemonSet(newObj, _ *appsv1.DaemonSet) error {
	webhook.LogMutation(logger, "DaemonSet", newObj.Namespace, newObj.Name)

	for i := range newObj.Spec.Template.Spec.Containers {
		if newObj.Spec.Template.Spec.Containers[i].Name == containerName {
			for j := range newObj.Spec.Template.Spec.Containers[i].Env {
				if newObj.Spec.Template.Spec.Containers[i].Env[j].Name == bpfDataIfacePatternEnvVariableName {
					newObj.Spec.Template.Spec.Containers[i].Env[j].Value = bpfDataIfacePatternValue
					return nil
				}
			}
			newObj.Spec.Template.Spec.Containers[i].Env = append(newObj.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
				Name:  bpfDataIfacePatternEnvVariableName,
				Value: bpfDataIfacePatternValue,
			})
			return nil
		}
	}

	return nil
}
