// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
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
	// ExtensionClasses defines the extension classes this extension is responsible for.
	ExtensionClasses []extensionsv1alpha1.ExtensionClass
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(_ context.Context, mgr manager.Manager, opts AddOptions) error {
	return backupbucket.Add(mgr, backupbucket.AddArgs{
		Actuator:          NewActuator(mgr),
		ControllerOptions: opts.Controller,
		Predicates:        getPredicates(opts),
		Type:              azure.Type,
		ExtensionClasses:  opts.ExtensionClasses,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}

func getPredicates(opts AddOptions) []predicate.Predicate {
	defaultPredicates := predicate.And(backupbucket.DefaultPredicates(opts.IgnoreOperationAnnotation)...)

	// Trigger reconcile if StorageAccountKeyMustRotate annotation is set to 'true' and the event is Create or Update.
	storageAccountKeyMustRotatePredicate := predicate.And(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		backupBucket, ok := obj.(*extensionsv1alpha1.BackupBucket)
		if !ok {
			return false
		}

		return kutil.HasMetaDataAnnotation(backupBucket, azure.StorageAccountKeyMustRotate, "true")
	}), predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update))

	return []predicate.Predicate{predicate.Or(defaultPredicates, storageAccountKeyMustRotatePredicate)}
}
