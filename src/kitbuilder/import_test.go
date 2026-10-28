package kitbuilder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/catalog"
	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/trust"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "catalog", "packages"), 0700); err != nil {
		t.Fatal(err)
	}
	index := domain.Index{Schema: 1, CatalogVersion: "2026.10.1", Packages: []string{},
		Profiles: []domain.Profile{{ID: "dev", Name: "Dev", Packages: []string{}}}}
	if err := atomicJSON(filepath.Join(root, "catalog", "catalog.json"), index); err != nil {
		t.Fatal(err)
	}
	return root
}
func inputFixture(t *testing.T, filename, body string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, filename)
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}
func addExample(t *testing.T, root string) domain.Package {
	t.Helper()
	p, err := Add(ImportOptions{
		Root: root, Input: inputFixture(t, "setup.exe", "fake-content-v1-not-executed"),
		ID: "example", Name: "Example software", Version: "1.0", Driver: "exe",
		Args: []string{"/S"}, DetectName: "Example*", Profiles: []string{"dev"},
		SourceURL: "https://vendor.invalid/example",
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestAddAndUpdateAreVersionedAndVerified(t *testing.T) {
	root := fixtureRoot(t)
	first := addExample(t, root)
	c, err := catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.Packages["example"].Version != "1.0" || len(c.Index.Profiles[0].Packages) != 1 {
		t.Fatalf("invalid catalog after add: %+v", c.Index)
	}
	if issues := catalog.Verify(root, c); len(issues) > 0 {
		t.Fatalf("invalid payload: %+v", issues)
	}
	updated, err := Update(UpdateOptions{
		Root: root, Input: inputFixture(t, "setup.exe", "fake-content-v2-not-executed"),
		ID: "example", Version: "2.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != "2.0" || updated.Payload.Path == first.Payload.Path {
		t.Fatalf("wrong immutable update: %+v", updated)
	}
	if updated.Detect.Rules[0].MinVersion != "2.0" {
		t.Fatal("detection version was not upgraded")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(first.Payload.Path))); err != nil {
		t.Fatalf("previous version unexpectedly removed: %v", err)
	}
	c, err = catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if issues := catalog.Verify(root, c); len(issues) > 0 {
		t.Fatalf("new payload not verified: %+v", issues)
	}
	if _, err := Update(UpdateOptions{Root: root, Input: inputFixture(t, "setup.exe", "downgrade"), ID: "example", Version: "1.9"}); err == nil {
		t.Fatal("downgrade accepted")
	}
}
func TestAddFailsWithoutSwitchOrDetection(t *testing.T) {
	root := fixtureRoot(t)
	input := inputFixture(t, "setup.exe", "fake")
	for _, o := range []ImportOptions{
		{Root: root, ID: "example", Name: "Test", Version: "1.0", Driver: "exe", Input: input, DetectName: "Test*"},
		{Root: root, ID: "example", Name: "Test", Version: "1.0", Driver: "exe", Input: input, Args: []string{"/S"}},
		{Root: root, ID: "example", Name: "Test", Version: "1.0", Driver: "exe", Input: input, Args: []string{"/S"}, DetectName: "Test*", Profiles: []string{"missing"}},
	} {
		if _, err := Add(o); err == nil {
			t.Fatalf("unsafe manifest accepted: %+v", o)
		}
	}
	c, err := catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Packages) != 0 {
		t.Fatalf("invalid add modified catalog: %+v", c)
	}
}
func TestVendorHashMismatchRejectedAndPayloadNotPublished(t *testing.T) {
	root := fixtureRoot(t)
	source := inputFixture(t, "setup.exe", "something")
	h := sha256.Sum256([]byte("different"))
	_, err := Add(ImportOptions{Root: root, Input: source, ID: "example", Name: "Example", Version: "1.0",
		Driver: "exe", Args: []string{"/S"}, DetectName: "Example*",
		VendorHash: hex.EncodeToString(h[:])})
	if err == nil {
		t.Fatal("vendor hash mismatch not rejected")
	}
	if _, err := os.Stat(filepath.Join(root, "payloads", "example", "1.0-setup.exe")); !os.IsNotExist(err) {
		t.Fatalf("payload left behind after rejected hash: %v", err)
	}
}
func TestSignedFolderKitAndTampering(t *testing.T) {
	root := fixtureRoot(t)
	p := addExample(t, root)
	keys := t.TempDir()
	privatePath := filepath.Join(keys, "signing-private.pem")
	publicPath := filepath.Join(keys, "signing-public.pem")
	if err := GenerateSigningKeys(root, privatePath, publicPath); err != nil {
		t.Fatal(err)
	}
	if err := GenerateSigningKeys(root, privatePath, publicPath); err == nil {
		t.Fatal("key overwritten")
	}
	pubFile, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := trust.ParsePublicPEM(pubFile)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "kit")
	compiler := func(root, out, key string) error {
		if key != hex.EncodeToString(pub) {
			t.Fatalf("missing pinned public key: %s", key)
		}
		return os.WriteFile(out, []byte("test-only engine fixture"), 0600)
	}
	report, err := buildFolder(BuildOptions{Root: root, Destination: dest, PrivateKeyPath: privatePath}, compiler)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Signed || report.PackageCount != 1 {
		t.Fatalf("bad build report %+v", report)
	}
	if err := trust.Verify(dest, pub); err != nil {
		t.Fatal(err)
	}
	copied, err := catalog.Load(dest)
	if err != nil {
		t.Fatal(err)
	}
	if issues := catalog.Verify(dest, copied); len(issues) != 0 {
		t.Fatalf("bad output payloads %+v", issues)
	}
	if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(p.Payload.Path))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "catalog", "catalog.sig")); !os.IsNotExist(err) {
		t.Fatal("builder modified source signature")
	}
	if _, err := os.Stat(filepath.Join(dest, "signing-private.pem")); !os.IsNotExist(err) {
		t.Fatal("private key in kit")
	}
	if _, err := buildFolder(BuildOptions{Root: root, Destination: dest, PrivateKeyPath: privatePath}, compiler); err == nil {
		t.Fatal("existing kit was overwritten")
	}
	manifest := filepath.Join(dest, "catalog", "packages", "example.json")
	bytes, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, append(bytes, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if err := trust.Verify(dest, pub); err == nil {
		t.Fatal("modified kit signature accepted")
	}
}
func TestUnsignedFolderRequiresExplicitLabFlag(t *testing.T) {
	root := fixtureRoot(t)
	addExample(t, root)
	dest := filepath.Join(t.TempDir(), "kit")
	fake := func(root, out, key string) error {
		if key != "" {
			t.Fatal("unsigned kit unexpectedly pinned a key")
		}
		return os.WriteFile(out, []byte("fake engine"), 0600)
	}
	if _, err := buildFolder(BuildOptions{Root: root, Destination: dest}, fake); err == nil {
		t.Fatal("unsigned kit accepted by default")
	}
	report, err := buildFolder(BuildOptions{Root: root, Destination: dest, LabUnsigned: true}, fake)
	if err != nil {
		t.Fatal(err)
	}
	if report.Signed {
		t.Fatal("lab kit claimed to be signed")
	}
	if _, err := os.Stat(filepath.Join(dest, "catalog", "catalog.sig")); !os.IsNotExist(err) {
		t.Fatal("unexpected signature")
	}
}
func TestKeysCannotBeStoredInRepo(t *testing.T) {
	root := fixtureRoot(t)
	external := t.TempDir()
	if err := GenerateSigningKeys(root, filepath.Join(root, "keys", "private.pem"), filepath.Join(external, "public.pem")); err == nil {
		t.Fatal("private key inside repo accepted")
	}
}
func TestManifestChecksRealHash(t *testing.T) {
	root := fixtureRoot(t)
	p := addExample(t, root)
	file := filepath.Join(root, filepath.FromSlash(p.Payload.Path))
	if err := os.WriteFile(file, []byte(strings.Repeat("Q", int(p.Payload.Size))), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := catalog.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	issues := catalog.Verify(root, c)
	if len(issues) != 1 || issues[0].Code != "E-VER-001" {
		t.Fatalf("expected tamper E-VER-001, got %+v", issues)
	}
}
func TestImportManifestJSONValid(t *testing.T) {
	root := fixtureRoot(t)
	addExample(t, root)
	raw, err := os.ReadFile(filepath.Join(root, "catalog", "packages", "example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["schema"] != float64(1) {
		t.Fatal("wrong schema")
	}
}
