package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedSchemasAreJSON(t *testing.T) {
	for _, name := range []string{"catalog.schema.json", "package.schema.json"} {
		file := filepath.Join("..", "..", "schema", name)
		contents, err := os.ReadFile(file)
		if err != nil { t.Fatal(err) }
		var body map[string]any
		if err := json.Unmarshal(contents, &body); err != nil { t.Fatalf("%s invalid: %v", file, err) }
		if body["$schema"] != "https://json-schema.org/draft/2020-12/schema" { t.Fatalf("%s: unexpected dialect", file) }
	}
}
