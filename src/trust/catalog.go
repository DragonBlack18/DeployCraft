// Package trust implements offline catalog authenticity using Ed25519.
// Payload SHA-256 values are covered transitively by the signed manifests.
// The trust anchor is a PUBLIC KEY pinned inside the DeployCraft executable.
package trust

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/catalog"
)

const signatureDomain = "DeployCraft/catalog/ed25519/v1\x00"
const digestDomain = "DeployCraft/catalog/tree/v1\x00"

// CatalogSignature is intentionally small and does not contain a public key.
// Accepting a public key supplied by the untrusted kit would defeat signing.
type CatalogSignature struct {
	Schema int `json:"schema"`
	Algorithm string `json:"algorithm"`
	KeyID string `json:"keyId"`
	DigestSHA256 string `json:"digestSha256"`
	SignatureBase64 string `json:"signatureBase64"`
}

// KeyID identifies the pinned signing key (not a secret).
func KeyID(pub ed25519.PublicKey) string {
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:8])
}
func signMessage(digest [sha256.Size]byte) []byte {
	out := make([]byte, 0, len(signatureDomain)+len(digest))
	out = append(out, []byte(signatureDomain)...)
	return append(out, digest[:]...)
}
func writeEntry(h hash.Hash, rel string, content []byte) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(rel)))
	h.Write(size[:])
	h.Write([]byte(rel))
	sum := sha256.Sum256(content)
	h.Write(sum[:])
}
func readCatalogFile(root, rel string) ([]byte, error) {
	if rel == "" || path.Clean(rel) != rel || path.IsAbs(rel) ||
		strings.ContainsAny(rel, "\\:") || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, fmt.Errorf("E-VER-004: caminho de catálogo inválido %q", rel)
	}
	abs, err := filepath.Abs(root)
	if err != nil { return nil, err }
	rootReal, err := filepath.EvalSymlinks(abs)
	if err != nil { return nil, err }
	file := filepath.Join(abs, filepath.FromSlash(rel))
	info, err := os.Lstat(file)
	if err != nil { return nil, err }
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("E-VER-004: arquivo de catálogo inválido %q", rel)
	}
	real, err := filepath.EvalSymlinks(file)
	if err != nil { return nil, err }
	back, err := filepath.Rel(rootReal, real)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("E-VER-004: catálogo fora do kit %q", rel)
	}
	return os.ReadFile(file)
}

// Digest signs only the index and the manifests that the loader actually uses.
// Paths are sorted and length-delimited, avoiding ambiguities and map order.
func Digest(root string) ([sha256.Size]byte, error) {
	var zero [sha256.Size]byte
	c, err := catalog.Load(root)
	if err != nil { return zero, err }
	files := []string{"catalog/catalog.json"}
	for _, rel := range c.Index.Packages {
		files = append(files, "catalog/"+rel)
	}
	sort.Strings(files)
	h := sha256.New()
	h.Write([]byte(digestDomain))
	for _, rel := range files {
		content, err := readCatalogFile(root, rel)
		if err != nil { return zero, err }
		writeEntry(h, rel, content)
	}
	copy(zero[:], h.Sum(nil))
	return zero, nil
}

// Sign returns the complete signature file; the private key never enters the kit.
func Sign(root string, priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("E-VER-004: chave privada Ed25519 inválida")
	}
	d, err := Digest(root)
	if err != nil { return nil, err }
	manifest := CatalogSignature{
		Schema: 1,
		Algorithm: "Ed25519",
		KeyID: KeyID(priv.Public().(ed25519.PublicKey)),
		DigestSHA256: hex.EncodeToString(d[:]),
		SignatureBase64: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signMessage(d))),
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil { return nil, err }
	return append(encoded, '\n'), nil
}

// Verify fails closed for missing, malformed, stale or wrong-key signatures.
func Verify(root string, pub ed25519.PublicKey) error {
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("E-VER-004: chave pública fixada no motor é inválida")
	}
	file := filepath.Join(root, "catalog", "catalog.sig")
	info, err := os.Lstat(file)
	if err != nil { return fmt.Errorf("E-VER-004: catalog.sig ausente: %w", err) }
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return errors.New("E-VER-004: catalog.sig inválido")
	}
	raw, err := os.ReadFile(file)
	if err != nil { return fmt.Errorf("E-VER-004: leitura de catalog.sig: %w", err) }
	var sig CatalogSignature
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sig); err != nil { return fmt.Errorf("E-VER-004: JSON de assinatura: %w", err) }
	var tail any
	if err := dec.Decode(&tail); err != io.EOF {
		return errors.New("E-VER-004: dados extras em catalog.sig")
	}
	if sig.Schema != 1 || sig.Algorithm != "Ed25519" || sig.KeyID != KeyID(pub) {
		return errors.New("E-VER-004: versão, algoritmo ou editor da assinatura inválido")
	}
	d, err := Digest(root)
	if err != nil { return fmt.Errorf("E-VER-004: catálogo não verificável: %w", err) }
	if sig.DigestSHA256 != hex.EncodeToString(d[:]) {
		return errors.New("E-VER-004: catálogo alterado após assinatura")
	}
	signature, err := base64.StdEncoding.DecodeString(sig.SignatureBase64)
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(pub, signMessage(d), signature) {
		return errors.New("E-VER-004: assinatura criptográfica inválida")
	}
	return nil
}
