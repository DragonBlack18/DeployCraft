package domain_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Domain source files must remain platform independent and free of I/O calls.
func TestDomainImportsRemainPure(t *testing.T) {
	allowed := map[string]bool{
		"context": true, "io": true, "encoding/json": true,
		"fmt": true, "math/big": true, "regexp": true, "strings": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil { t.Fatal(err) }
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") ||
			strings.HasSuffix(ent.Name(), "_test.go") { continue }
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(".", ent.Name()), nil, parser.ImportsOnly)
		if err != nil { t.Fatal(err) }
		for _, imp := range f.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil { t.Fatal(err) }
			if !allowed[name] { t.Fatalf("%s imports forbidden package %q", ent.Name(), name) }
		}
	}
}
