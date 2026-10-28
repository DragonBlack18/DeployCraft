package kitbuilder

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledWindowsEnginePinsCatalogPublicKey(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "DeployCraft.exe")
	publicHex := strings.Repeat("a1", 32)
	if err := compileWindowsEngine(root, target, publicHex); err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(binary) < 100*1024 || !bytes.HasPrefix(binary, []byte("MZ")) {
		t.Fatal("motor Windows inválido")
	}
	if !bytes.Contains(binary, []byte(publicHex)) {
		t.Fatal("chave pública não está fixada no motor Windows")
	}
}
