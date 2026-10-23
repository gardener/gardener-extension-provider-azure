// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package apitypecheck implements an analyzer that verifies every exported field
// of an internal API types package is referenced by at least one field.Child(...)
// call in the sibling validation package.
//
// The analyzer runs on validation packages (import path contains "/validation").
// It locates the parent types package (same import path prefix, without "/validation"),
// collects all exported struct fields whose owning struct is not on the exclusion list,
// and checks that each field's lowerCamelCase name appears in at least one
// field.Child("name") call within the validation package.
//
// Structs whose names end with "Status" or "State", as well as structs listed in
// noValidationStructs, are skipped — they are output or lookup types that do not
// require input validation.
// Fields listed in noValidationFields are also skipped.
package apitypecheck

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer reports exported API type fields that have no corresponding
// field.Child("name") reference in the enclosing validation package.
var Analyzer = &analysis.Analyzer{
	Name:     "apitypecheck",
	Doc:      "check that every exported field of an internal API types package is referenced in the sibling validation package",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// noValidationStructs lists struct type names that do not require input
// validation (output-only, status, or pure-lookup types).
// Struct names ending in "Status" or "State" are excluded automatically; this
// map covers additional cases that do not follow that naming convention.
var noValidationStructs = map[string]bool{
	// infrastructure — output/status types
	"InfrastructureStatus": true,
	"NetworkStatus":        true,
	"Subnet":               true,
	"RouteTable":           true,
	"SecurityGroup":        true,
	"VNetStatus":           true,
	"IdentityStatus":       true,
	"AzureResource":        true,
	// worker — output/status types
	"WorkerStatus":  true,
	"MachineImage":  true,
	"VmoDependency": true,
	// cloudprofile — lookup / informational only
	"CloudProfileConfig":  true,
	"MachineImages":       true,
	"MachineImageVersion": true,
	"MachineImageFlavor":  true,
	"MachineType":         true,
	"DomainCount":         true,
	"Image":               true,
}

// noValidationFields lists "StructName.fieldName" pairs (lowerCamelCase field)
// that are intentionally not validated (pure boolean toggles, embedded meta,
// pass-through fields, or fields with non-standard field path names that
// diverge from the Go field name).
var noValidationFields = map[string]bool{
	// embedded Kubernetes meta — validated by the API machinery
	"WorkerConfig.typeMeta":           true,
	"WorkerStatus.typeMeta":           true,
	"InfrastructureConfig.typeMeta":   true,
	"InfrastructureStatus.typeMeta":   true,
	"InfrastructureState.typeMeta":    true,
	"ControlPlaneConfig.typeMeta":     true,
	"BackupBucketConfig.typeMeta":     true,
	"WorkloadIdentityConfig.typeMeta": true,
	"CloudProfileConfig.typeMeta":     true,

	// CloudControllerManagerConfig.featureGates — map of arbitrary feature gate
	// names validated by the upstream Gardener admission plugin, not here.
	"CloudControllerManagerConfig.featureGates": true,

	// NodeTemplate — passed through verbatim from the extensions API, not validated here.
	"WorkerConfig.nodeTemplate": true,

	// Pure boolean toggles with no range/format constraint.
	"NatGatewayConfig.enabled":      true,
	"ZonedNatGatewayConfig.enabled": true,
	"DiagnosticsProfile.enabled":    true,
	"ImmutableConfig.locked":        true,
	"InfrastructureConfig.zoned":    true,

	// Storage boolean toggles — no range constraint, just on/off.
	"Storage.managedDefaultStorageClass":        true,
	"Storage.managedDefaultVolumeSnapshotClass": true,

	// ControlPlaneConfig.storage — sub-struct containing only boolean toggles.
	"ControlPlaneConfig.storage": true,

	// IdentityConfig.acrAccess — optional boolean flag, no constraint.
	"IdentityConfig.acrAccess": true,

	// NetworkConfig.vNet is validated, but the field path used is "vnet" (all
	// lowercase) while the Go field name VNet lowerCamelCase is "vNet".
	"NetworkConfig.vNet": true,

	// VNet.dDosProtectionPlanID is validated, but the field path used is
	// "ddosProtectionPlanID" while Go field DDosProtectionPlanID → "dDosProtectionPlanID".
	"VNet.dDosProtectionPlanID": true,

	// BackupBucketConfig.cloudConfiguration — CloudConfiguration contains only
	// a Name string (cloud selector); the allowed values are enforced at the
	// cloudprofile level, not in the backup-bucket validation.
	"BackupBucketConfig.cloudConfiguration": true,
}

func run(pass *analysis.Pass) (any, error) {
	// Only run on packages whose import path contains "/validation".
	// Skip external test packages (path ends in "_test" or ".test") — they don't
	// contribute validation logic and would generate spurious diagnostics.
	pkgPath := pass.Pkg.Path()
	if strings.HasSuffix(pkgPath, "_test") || strings.HasSuffix(pkgPath, ".test") {
		return nil, nil
	}
	validationIdx := strings.LastIndex(pkgPath, "/validation")
	if validationIdx < 0 {
		return nil, nil
	}

	// Derive the sibling types package path: everything before "/validation".
	typesPkgPath := pkgPath[:validationIdx]

	// Find the types package among the imports of the validation package.
	typesPkg := findPkg(pass.Pkg, typesPkgPath)
	if typesPkg == nil {
		pass.Reportf(pass.Files[0].Package,
			"validation package %s does not import sibling types package %s",
			pass.Pkg.Path(), typesPkgPath)
		return nil, nil
	}

	// Collect all field.Child("name") string literals in the validation package.
	validatedFields := collectValidatedFields(pass)

	// Walk exported structs in the types package and check coverage.
	checkTypesCoverage(pass, typesPkg, validatedFields)

	return nil, nil
}

// findPkg searches the immediate imports of pkg for a package with the given path.
func findPkg(pkg *types.Package, path string) *types.Package {
	for _, imp := range pkg.Imports() {
		if imp.Path() == path {
			return imp
		}
	}
	return nil
}

// isFieldPath reports whether expr has type *field.Path from
// k8s.io/apimachinery/pkg/util/validation/field.
func isFieldPath(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Name() == "Path" &&
		obj.Pkg() != nil &&
		obj.Pkg().Path() == "k8s.io/apimachinery/pkg/util/validation/field"
}

// collectValidatedFields returns the set of field names (lowerCamelCase) that
// appear as the first string literal argument of field.Child(...) calls within
// the current package.
//
// Known limitation: the set is package-wide, not per-type. A Child("tags") call
// inside ValidateOtherConfig satisfies the check for WidgetConfig.Tags as well.
// This is acceptable because the Azure validation package has one function per
// config type and no two checked types share a field name that one validates and
// the other does not.
func collectValidatedFields(pass *analysis.Pass) map[string]bool {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	validated := make(map[string]bool)

	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call := n.(*ast.CallExpr)

		// We want calls of the form <expr>.Child("name", ...) where the
		// receiver is *field.Path from k8s.io/apimachinery.
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Child" {
			return
		}
		if !isFieldPath(pass, sel.X) {
			return
		}
		// Record only the first argument — that is the immediate field name.
		// Variadic extras describe nested path segments, not top-level fields.
		if len(call.Args) == 0 {
			return
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return
		}
		fieldName, err := strconv.Unquote(lit.Value)
		if err != nil {
			return
		}
		validated[fieldName] = true
	})

	return validated
}

// checkTypesCoverage iterates over all exported struct fields in typesPkg and
// reports any field whose lowerCamelCase name is absent from validatedFields.
func checkTypesCoverage(pass *analysis.Pass, typesPkg *types.Package, validatedFields map[string]bool) {
	scope := typesPkg.Scope()

	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}

		typeName, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			continue
		}
		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}

		structName := named.Obj().Name()
		if noValidationStructs[structName] {
			continue
		}
		// Auto-exclude output-only types by naming convention.
		if strings.HasSuffix(structName, "Status") || strings.HasSuffix(structName, "State") {
			continue
		}

		for i := range st.NumFields() {
			field := st.Field(i)
			if !field.Exported() {
				continue
			}

			camel := lowerCamelCase(field.Name())

			// Skip embedded fields from external packages (e.g. metav1.TypeMeta).
			if field.Embedded() {
				pkg := fieldPkgPath(field)
				if pkg != typesPkg.Path() {
					continue
				}
			}

			key := structName + "." + camel
			if noValidationFields[key] {
				continue
			}

			if !validatedFields[camel] {
				// Report at the package declaration of the first validation file so
				// that analysistest `// want` annotations can be placed there.
				pos := pass.Files[0].Package
				pass.Reportf(pos,
					"field %s.%s (%s) has no field.Child(%q) reference in validation package %s",
					structName, field.Name(), typesPkg.Path(), camel, pass.Pkg.Path(),
				)
			}
		}
	}
}

// fieldPkgPath returns the package path of a struct field's type, dereferencing
// pointers and slices as needed. Returns empty string for built-in types.
func fieldPkgPath(field *types.Var) string {
	t := field.Type()
	// Unwrap pointer / slice.
	for {
		switch u := t.(type) {
		case *types.Pointer:
			t = u.Elem()
		case *types.Slice:
			t = u.Elem()
		default:
			goto done
		}
	}
done:
	if named, ok := t.(*types.Named); ok && named.Obj().Pkg() != nil {
		return named.Obj().Pkg().Path()
	}
	return ""
}

// lowerCamelCase converts a Go exported identifier to the lowerCamelCase form
// used in field.Child("name") calls and json tags.
//
// Go convention: consecutive uppercase letters that form an acronym at the
// start of the name are all lowercased (e.g. "VPC" → "vpc", "MTU" → "mtu",
// "CSIFilestore" → "csiFilestore"). A single leading uppercase letter is just
// lowercased (e.g. "Color" → "color").
func lowerCamelCase(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	// Count leading uppercase letters.
	i := 0
	for i < len(r) && unicode.IsUpper(r[i]) {
		i++
	}
	if i == 0 {
		return s
	}
	// If the whole string is uppercase, lowercase it all.
	if i == len(r) {
		return strings.ToLower(s)
	}
	// If only one leading uppercase, lowercase it.
	if i == 1 {
		r[0] = unicode.ToLower(r[0])
		return string(r)
	}
	// Multiple leading uppercase letters: all but the last are the acronym
	// (the last uppercase letter starts the next word, e.g. "CSIFilestore" →
	// lowercase "csi", keep "F" as start of "Filestore").
	acronymEnd := i - 1
	for j := 0; j < acronymEnd; j++ {
		r[j] = unicode.ToLower(r[j])
	}
	return string(r)
}
