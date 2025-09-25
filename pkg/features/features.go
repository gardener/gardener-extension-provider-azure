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
}
