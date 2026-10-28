// Package pack implements the uncompressed, indexed, append-only SFX format.
// It never extracts a whole archive or executes a process.
package pack

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	magic         = "DCPACK01"
	footerSize    = 56
	maxIndexBytes = 4 << 20
)

type Entry struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type Index struct {
	Schema int     `json:"schema"`
	Files  []Entry `json:"files"`
}

func safePath(rel string) error {
	if rel == "" || rel == "." || strings.ContainsAny(rel, "\\:\x00") ||
		strings.HasPrefix(rel, "/") || path.Clean(rel) != rel ||
		strings.HasPrefix(rel, "../") || rel == ".." {
		return fmt.Errorf("E-SFX-001: caminho inseguro %q", rel)
	}
	return nil
}
func contained(root, file string) bool {
	rel, err := filepath.Rel(root, file)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}
func sourceFile(root, rel string) (*os.File, os.FileInfo, error) {
	if err := safePath(rel); err != nil {
		return nil, nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	base, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, nil, err
	}
	source := filepath.Join(abs, filepath.FromSlash(rel))
	info, err := os.Lstat(source)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return nil, nil, fmt.Errorf("E-SFX-001: arquivo ausente, vazio ou não regular: %s", rel)
	}
	real, err := filepath.EvalSymlinks(source)
	if err != nil {
		return nil, nil, err
	}
	if !contained(base, real) {
		return nil, nil, fmt.Errorf("E-SFX-001: arquivo fora do kit: %s", rel)
	}
	f, err := os.Open(source)
	if err != nil {
		return nil, nil, err
	}
	return f, info, nil
}

// Append writes payload entries, a JSON index and a fixed footer after an EXE.
// The output must already contain the unsigned motor. Sign with Authenticode
// ONLY after the append operation is complete.
func Append(out *os.File, root string, paths []string) (Index, error) {
	index := Index{Schema: 1, Files: []Entry{}}
	if len(paths) == 0 {
		return index, errors.New("E-SFX-001: pacote vazio")
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	seen := map[string]bool{}
	for _, rel := range sorted {
		if err := safePath(rel); err != nil {
			return index, err
		}
		folded := strings.ToLower(rel)
		if seen[folded] {
			return index, fmt.Errorf("E-SFX-001: caminho repetido %s", rel)
		}
		seen[folded] = true
	}
	for _, rel := range sorted {
		source, info, err := sourceFile(root, rel)
		if err != nil {
			return index, err
		}
		offset, err := out.Seek(0, io.SeekEnd)
		if err != nil {
			source.Close()
			return index, err
		}
		hash := sha256.New()
		n, copyErr := io.CopyBuffer(io.MultiWriter(out, hash), source, make([]byte, 1<<20))
		closeErr := source.Close()
		if copyErr != nil {
			return index, copyErr
		}
		if closeErr != nil {
			return index, closeErr
		}
		if n != info.Size() {
			return index, fmt.Errorf("E-SFX-002: arquivo alterado durante cópia %s", rel)
		}
		index.Files = append(index.Files, Entry{Path: rel, Offset: offset, Size: n, SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	indexStart, err := out.Seek(0, io.SeekEnd)
	if err != nil {
		return index, err
	}
	data, err := json.Marshal(index)
	if err != nil {
		return index, err
	}
	if len(data) > maxIndexBytes {
		return index, fmt.Errorf("E-SFX-001: índice excede 4 MiB")
	}
	if _, err := out.Write(data); err != nil {
		return index, err
	}
	var footer [footerSize]byte
	copy(footer[:8], magic)
	binary.LittleEndian.PutUint64(footer[8:16], uint64(indexStart))
	binary.LittleEndian.PutUint64(footer[16:24], uint64(len(data)))
	sum := sha256.Sum256(data)
	copy(footer[24:], sum[:])
	if _, err := out.Write(footer[:]); err != nil {
		return index, err
	}
	if err := out.Sync(); err != nil {
		return index, err
	}
	return index, nil
}
