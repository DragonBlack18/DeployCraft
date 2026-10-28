package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

type VerifyIssue struct {
	PackageID string `json:"packageId"`
	Path      string `json:"path"`
	Code      string `json:"code"`
	Problem   string `json:"problem"`
}

// Verify checks the actual bytes of all catalogued offline payloads.
// This never executes an installer and never modifies the kit.
func Verify(root string, c domain.Catalog) []VerifyIssue {
	ids := make([]string, 0, len(c.Packages))
	for id := range c.Packages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	issues := []VerifyIssue{}
	for _, id := range ids {
		p := c.Packages[id]
		if p.Payload.Kind == "dir" {
			for _, f := range p.Payload.Files {
				rel := p.Payload.Path + "/" + f.Path
				if issue := verifyFile(root, id, rel, f.Size, f.SHA256); issue != nil {
					issues = append(issues, *issue)
				}
			}
		} else {
			if issue := verifyFile(root, id, p.Payload.Path, p.Payload.Size, p.Payload.SHA256); issue != nil {
				issues = append(issues, *issue)
			}
		}
	}
	return issues
}
func verifyFile(root, id, rel string, size int64, wantSHA string) *VerifyIssue {
	fail := func(code, msg string) *VerifyIssue {
		return &VerifyIssue{PackageID: id, Path: rel, Code: code, Problem: msg}
	}
	if err := safeRelative(rel); err != nil {
		return fail("E-VER-003", err.Error())
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	full := filepath.Join(rootAbs, filepath.FromSlash(rel))
	info, err := os.Lstat(full)
	if err != nil {
		return fail("E-VER-003", fmt.Sprintf("payload ausente: %v", err))
	}
	if !info.Mode().IsRegular() {
		return fail("E-VER-003", "payload não é arquivo regular")
	}
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	back, err := filepath.Rel(rootReal, real)
	if err != nil || back == ".." || len(back) >= 3 && back[:3] == ".."+string(os.PathSeparator) {
		return fail("E-VER-003", "payload fora da raiz do kit")
	}
	if info.Size() != size {
		return fail("E-VER-002", fmt.Sprintf("tamanho esperado %d, obtido %d", size, info.Size()))
	}
	f, err := os.Open(full)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyBuffer(h, f, make([]byte, 1<<20)); err != nil {
		return fail("E-VER-003", err.Error())
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !equalHash(actual, wantSHA) {
		return fail("E-VER-001", fmt.Sprintf("SHA-256 esperado %s, obtido %s", wantSHA, actual))
	}
	return nil
}
func equalHash(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'F' {
			x += 32
		}
		if y >= 'A' && y <= 'F' {
			y += 32
		}
		if x != y {
			return false
		}
	}
	return true
}
