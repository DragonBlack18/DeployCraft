// Package catalog validates a versioned, offline-first manifest collection.
package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

const maxJSONBytes int64 = 1 << 20

// safeRelative forbids absolute paths and traversal on any host OS.
func safeRelative(s string) error {
	if s == "" || s == "." || strings.ContainsAny(s, "\\:") || strings.HasPrefix(s, "/") ||
		path.Clean(s) != s || s == ".." || strings.HasPrefix(s, "../") {
		return fmt.Errorf("E-CAT-003: caminho não seguro %q", s)
	}
	return nil
}
func readStrict(file string, out any) error {
	info, err := os.Lstat(file)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxJSONBytes {
		return fmt.Errorf("E-CAT-003 %s: arquivo inválido ou maior que 1 MiB", file)
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, maxJSONBytes+1))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("E-CAT-001 %s: %w", file, err)
	}
	var rest any
	if err := d.Decode(&rest); err != io.EOF {
		return fmt.Errorf("E-CAT-001 %s: JSON contém dados adicionais", file)
	}
	return nil
}

// Load reads catalog/catalog.json and every referenced package exactly once.
func Load(root string) (domain.Catalog, error) {
	c := domain.Catalog{Packages: map[string]domain.Package{}, Profiles: map[string]domain.Profile{}}
	var idx domain.Index
	if err := readStrict(filepath.Join(root, "catalog", "catalog.json"), &idx); err != nil {
		return c, err
	}
	if idx.Schema != 1 || !domain.ValidVersion(idx.CatalogVersion) {
		return c, fmt.Errorf("E-CAT-003 catalog/catalog.json: schema ou catalogVersion inválido")
	}
	seenFiles := map[string]bool{}
	for _, rel := range idx.Packages {
		if err := safeRelative(rel); err != nil {
			return c, err
		}
		if !strings.HasPrefix(rel, "packages/") || !strings.HasSuffix(rel, ".json") || seenFiles[rel] {
			return c, fmt.Errorf("E-CAT-003 catalog: entrada duplicada ou fora de packages/: %q", rel)
		}
		seenFiles[rel] = true
		var p domain.Package
		if err := readStrict(filepath.Join(root, "catalog", filepath.FromSlash(rel)), &p); err != nil {
			return c, err
		}
		if err := validatePackage(p); err != nil {
			return c, fmt.Errorf("catalog/%s: %w", rel, err)
		}
		if path.Base(rel) != p.ID+".json" {
			return c, fmt.Errorf("E-CAT-005 %s: arquivo não corresponde ao id %s", rel, p.ID)
		}
		if _, exists := c.Packages[p.ID]; exists {
			return c, fmt.Errorf("E-CAT-005: id duplicado %q", p.ID)
		}
		c.Packages[p.ID] = p
	}
	for _, p := range idx.Profiles {
		if !validID(p.ID) || strings.TrimSpace(p.Name) == "" {
			return c, fmt.Errorf("E-CAT-003: perfil inválido %q", p.ID)
		}
		if _, exists := c.Profiles[p.ID]; exists {
			return c, fmt.Errorf("E-CAT-005: perfil duplicado %s", p.ID)
		}
		c.Profiles[p.ID] = p
	}
	c.Index = idx
	if err := validateReferences(c); err != nil {
		return c, err
	}
	return c, nil
}
