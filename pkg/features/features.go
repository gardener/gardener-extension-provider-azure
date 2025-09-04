package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// DisableRemedyController controls whether the azure provider will disable the remedy-controller. Technically it will still be deployed, but scaled down to zero.
	// alpha: v1.29.0
	DisableRemedyController featuregate.Feature = "DisableRemedyController"
	// EnableImmutableBuckets controls whether the controller would react to immutable bucket configuration. Extra permissions from Azure are necessary for this feature to work.
	// alpha: v1.52.0
	EnableImmutableBuckets featuregate.Feature = "EnableImmutableBuckets"
	// ForceAvailabilitySetMigration controls whether the controller will force the migration of existing availability sets to virtual machine scale sets.
	// alpha: v1.54.0
	ForceAvailabilitySetMigration featuregate.Feature = "ForceAvailabilitySetMigration"
	// ForceNatGateway controls whether the controller will force the creation of a NAT gateway for new shoot cluster if the NAT-Gateway is not explicitly set and a user does not bring his own VNet.
	// Necessary because Azure deprecated default outbound access https://azure.microsoft.com/en-us/updates?id=default-outbound-access-for-vms-in-azure-will-be-retired-transition-to-a-new-method-of-internet-access
	// alpha: v1.54.0
	ForceNatGateway featuregate.Feature = "ForceNatGateway"
)

// ExtensionFeatureGate is the feature gate for the extension controllers.
var ExtensionFeatureGate = featuregate.NewFeatureGate()

func init() {
	RegisterExtensionFeatureGate()
}

// RegisterExtensionFeatureGate registers features to the extension feature gate.
func RegisterExtensionFeatureGate() {
	runtime.Must(ExtensionFeatureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		DisableRemedyController: {Default: false, PreRelease: featuregate.Alpha},
	}))
	runtime.Must(ExtensionFeatureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		EnableImmutableBuckets: {Default: false, PreRelease: featuregate.Alpha},
	}))
	runtime.Must(ExtensionFeatureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		ForceAvailabilitySetMigration: {Default: false, PreRelease: featuregate.Alpha},
	}))
	runtime.Must(ExtensionFeatureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		ForceNatGateway: {Default: false, PreRelease: featuregate.Alpha},
	}))
}
