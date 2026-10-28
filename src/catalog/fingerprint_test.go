package catalog

import (
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

func TestFingerprintStableAcrossMapOrderingAndDetectsContentChanges(t *testing.T) {
	index := domain.Index{Schema: 1, CatalogVersion: "2026.10.1", Packages: []string{"packages/a.json", "packages/b.json"}}
	a := domain.Package{ID: "a", Version: "1.0", Name: "A"}
	b := domain.Package{ID: "b", Version: "2.0", Name: "B", DependsOn: []string{"a"}}
	first := domain.Catalog{Index: index, Packages: map[string]domain.Package{"a": a, "b": b}}
	second := domain.Catalog{Index: index, Packages: map[string]domain.Package{}}
	second.Packages["b"] = b
	second.Packages["a"] = a
	f1, err := Fingerprint(first)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := Fingerprint(second)
	if err != nil {
		t.Fatal(err)
	}
	if f1 != f2 || len(f1) != 64 {
		t.Fatalf("unstable fingerprints %s %s", f1, f2)
	}
	b.Version = "2.1"
	second.Packages["b"] = b
	f3, err := Fingerprint(second)
	if err != nil {
		t.Fatal(err)
	}
	if f1 == f3 {
		t.Fatal("changed package version accepted")
	}
	second.Packages["b"] = domain.Package{ID: "b", Name: "B", Version: "2.0", DependsOn: []string{"a"}, ConflictsWith: []string{"c"}}
	f4, err := Fingerprint(second)
	if err != nil {
		t.Fatal(err)
	}
	if f1 == f4 {
		t.Fatal("changed dependency/conflict accepted")
	}
}
func TestFingerprintRejectsMismatchedMapID(t *testing.T) {
	c := domain.Catalog{Packages: map[string]domain.Package{"a": {ID: "other"}}}
	if _, err := Fingerprint(c); err == nil {
		t.Fatal("mismatched map ID accepted")
	}
}

func TestFingerprintIncludesEffectiveProfileMap(t *testing.T) {
	c := domain.Catalog{
		Index:    domain.Index{Schema: 1, CatalogVersion: "2026.10.1"},
		Packages: map[string]domain.Package{},
		Profiles: map[string]domain.Profile{
			"dev": {ID: "dev", Name: "Developer", Packages: []string{"alpha"}},
		},
	}
	first, err := Fingerprint(c)
	if err != nil {
		t.Fatal(err)
	}
	profile := c.Profiles["dev"]
	profile.Packages = []string{"beta"}
	c.Profiles["dev"] = profile
	second, err := Fingerprint(c)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("modified effective profile did not change digest")
	}
}
