// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	backupbucketcontroller "github.com/gardener/gardener/pkg/gardenlet/controller/backupbucket"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the Azure backupbucket controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	return backupbucket.Add(mgr, backupbucket.AddArgs{
		Actuator:          NewActuator(mgr),
		ControllerOptions: opts.Controller,
		Predicates:        getPredicates(ctx, mgr.GetClient(), opts),
		Type:              azure.Type,
		ExtensionClass:    opts.ExtensionClass,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}

func getPredicates(ctx context.Context, c client.Client, opts AddOptions) []predicate.Predicate {
	// Keep old behavior by doing a logical And of the default predicates.
	defaultPredicates := predicate.And(backupbucket.DefaultPredicates(opts.IgnoreOperationAnnotation)...)

	// TODO(shafeeqes): Remove this in a future release when all generated secrets have the annotation set.
	generatedSecretRefPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		backupBucket, ok := obj.(*extensionsv1alpha1.BackupBucket)
		if !ok {
			return false
		}

		if backupBucket.Status.GeneratedSecretRef == nil {
			return false
		}

		generatedSecret, err := kubernetesutils.GetSecretByReference(ctx, c, backupBucket.Status.GeneratedSecretRef)
		if err != nil {
			return false
		}

		return generatedSecret != nil && !metav1.HasAnnotation(generatedSecret.ObjectMeta, backupbucketcontroller.RenewKeyTimeStampAnnotation)
	})

	return []predicate.Predicate{predicate.Or(defaultPredicates, generatedSecretRefPredicate)}
}
