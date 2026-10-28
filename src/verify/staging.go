// Package verify copies trusted byte streams into an isolated staging directory,
// calculating SHA-256 during the copy. It never runs payload files.
package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

type IntegrityError struct {
	Code  string
	Path  string
	Cause string
}

func (e *IntegrityError) Error() string { return fmt.Sprintf("%s %s: %s", e.Code, e.Path, e.Cause) }

func isContained(root, child string) bool {
	rel, err := filepath.Rel(root, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}
func safeSource(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" ||
		strings.Contains(relative, "\\") || strings.Contains(relative, ":") ||
		path.Clean(relative) != relative || strings.HasPrefix(relative, "..") {
		return "", fmt.Errorf("caminho de payload inválido")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(rootAbs, filepath.FromSlash(relative))
	if !isContained(rootAbs, joined) {
		return "", fmt.Errorf("payload fora do kit")
	}
	info, err := os.Lstat(joined)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("payload não é arquivo regular")
	}
	real, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", err
	}
	if !isContained(rootReal, real) {
		return "", fmt.Errorf("payload aponta para fora do kit")
	}
	return joined, nil
}
func copyOne(root, rel, destination, wantHash string, wantSize int64) error {
	fail := func(code, message string) error { return &IntegrityError{Code: code, Path: rel, Cause: message} }
	source, err := safeSource(root, rel)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	in, err := os.Open(source)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	if info.Size() != wantSize {
		return fail("E-VER-002", fmt.Sprintf("tamanho esperado %d, obtido %d", wantSize, info.Size()))
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return fail("E-VER-003", err.Error())
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	hasher := sha256.New()
	n, copyErr := io.CopyBuffer(io.MultiWriter(out, hasher), in, make([]byte, 1<<20))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(destination)
		return fail("E-VER-003", "cópia incompleta")
	}
	if n != wantSize {
		_ = os.Remove(destination)
		return fail("E-VER-002", "tamanho alterado durante a cópia")
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actual, wantHash) {
		_ = os.Remove(destination)
		return fail("E-VER-001", fmt.Sprintf("SHA-256 esperado %s, obtido %s", wantHash, actual))
	}
	return nil
}

// Stage copies an MSI/EXE single-file or directory payload into a unique,
// newly created package directory. The caller is responsible for cleanup.
func Stage(kitRoot, runRoot string, p domain.Package) (string, error) {
	if p.Install.Driver != "msi" && p.Install.Driver != "exe" {
		return "", fmt.Errorf("E-DRV-002: driver ainda não executável nesta fase")
	}
	if p.ID == "" || strings.ContainsAny(p.ID, "/\\.:") || strings.HasPrefix(p.ID, "-") {
		return "", fmt.Errorf("E-CAT-003: package ID inválido")
	}
	if err := os.MkdirAll(runRoot, 0700); err != nil {
		return "", err
	}
	packageRoot := filepath.Join(runRoot, p.ID)
	if err := os.Mkdir(packageRoot, 0700); err != nil {
		return "", err
	}
	if p.Payload.Kind == "file" {
		name := filepath.Base(filepath.FromSlash(p.Payload.Path))
		dst := filepath.Join(packageRoot, name)
		if err := copyOne(kitRoot, p.Payload.Path, dst, p.Payload.SHA256, p.Payload.Size); err != nil {
			return "", err
		}
		return dst, nil
	}
	if p.Payload.Kind == "dir" {
		found := false
		for _, f := range p.Payload.Files {
			rel := p.Payload.Path + "/" + f.Path
			relativeFile := filepath.FromSlash(f.Path)
			target := filepath.Join(packageRoot, relativeFile)
			if !isContained(packageRoot, target) {
				return "", fmt.Errorf("E-CAT-003: entrada fora do staging")
			}
			if err := copyOne(kitRoot, rel, target, f.SHA256, f.Size); err != nil {
				return "", err
			}
			if f.Path == p.Payload.Entry {
				found = true
			}
		}
		if !found {
			return "", fmt.Errorf("E-CAT-003: entry ausente dos arquivos do payload")
		}
		return filepath.Join(packageRoot, filepath.FromSlash(p.Payload.Entry)), nil
	}
	return "", fmt.Errorf("E-DRV-002: kind %s não suportado para %s", p.Payload.Kind, p.Install.Driver)
}
