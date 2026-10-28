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

func stageSourceFile(source domain.PayloadSource, rel, destination string, expectedSize int64, expectedHash string) error {
	fail := func(code, reason string) error { return &IntegrityError{Code: code, Path: rel, Cause: reason} }
	if rel == "" || path.Clean(rel) != rel || strings.HasPrefix(rel, "/") ||
		strings.ContainsAny(rel, "\\:") || strings.HasPrefix(rel, "../") || rel == ".." {
		return fail("E-VER-003", "caminho do payload inválido")
	}
	f, err := source.Open(rel)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	defer f.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return fail("E-VER-003", err.Error())
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fail("E-VER-003", err.Error())
	}
	hash := sha256.New()
	size, copyErr := io.CopyBuffer(io.MultiWriter(out, hash), f, make([]byte, 1<<20))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(destination)
		return fail("E-VER-003", "cópia do payload falhou")
	}
	if size != expectedSize {
		os.Remove(destination)
		return fail("E-VER-002", "tamanho do payload inválido")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedHash) {
		os.Remove(destination)
		return fail("E-VER-001", "SHA-256 do payload divergente")
	}
	return nil
}

// StageSource stages a selected MSI/EXE from the appended SFX PackSource.
// Only the selected payload is copied; unrelated packages remain inside the SFX.
func StageSource(src domain.PayloadSource, runRoot string, p domain.Package) (string, error) {
	if src == nil {
		return "", fmt.Errorf("E-VER-003: fonte ausente")
	}
	if p.Install.Driver != "msi" && p.Install.Driver != "exe" {
		return "", fmt.Errorf("E-DRV-002: driver indisponível para staging de fonte")
	}
	if p.ID == "" || strings.ContainsAny(p.ID, "/\\:.") || strings.HasPrefix(p.ID, "-") {
		return "", fmt.Errorf("E-CAT-003: id inválido")
	}
	if err := os.MkdirAll(runRoot, 0700); err != nil {
		return "", err
	}
	pkgRoot := filepath.Join(runRoot, p.ID)
	if err := os.Mkdir(pkgRoot, 0700); err != nil {
		return "", err
	}
	switch p.Payload.Kind {
	case "file":
		name := path.Base(p.Payload.Path)
		dst := filepath.Join(pkgRoot, name)
		if err := stageSourceFile(src, p.Payload.Path, dst, p.Payload.Size, p.Payload.SHA256); err != nil {
			return "", err
		}
		return dst, nil
	case "dir":
		entryFound := false
		for _, f := range p.Payload.Files {
			if f.Path == p.Payload.Entry {
				entryFound = true
			}
			if f.Path == "" || path.Clean(f.Path) != f.Path || strings.HasPrefix(f.Path, "../") || strings.HasPrefix(f.Path, "/") {
				return "", fmt.Errorf("E-CAT-003: entrada de diretório insegura")
			}
			dst := filepath.Join(pkgRoot, filepath.FromSlash(f.Path))
			if !isContained(pkgRoot, dst) {
				return "", fmt.Errorf("E-CAT-003: tentativa de escapar do staging")
			}
			if err := stageSourceFile(src, p.Payload.Path+"/"+f.Path, dst, f.Size, f.SHA256); err != nil {
				return "", err
			}
		}
		if !entryFound {
			return "", fmt.Errorf("E-CAT-003: entry ausente do manifesto")
		}
		return filepath.Join(pkgRoot, filepath.FromSlash(p.Payload.Entry)), nil
	default:
		return "", fmt.Errorf("E-DRV-002: payload.kind não suportado")
	}
}
