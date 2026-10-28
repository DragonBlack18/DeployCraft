package kitbuilder

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/DragonBlack18/DeployCraft/src/catalog"
	"github.com/DragonBlack18/DeployCraft/src/trust"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BuildOptions creates a new, immutable destination folder.
type BuildOptions struct {
	Root, Destination, PrivateKeyPath, SourceRoot string
	LabUnsigned                                   bool
}
type BuildReport struct {
	Destination    string
	CatalogVersion string
	PackageCount   int
	Signed         bool
	PublicKeyHex   string
	EngineSHA256   string
}

func inside(root, child string) bool {
	back, err := filepath.Rel(root, child)
	return err == nil && back != ".." && !strings.HasPrefix(back, ".."+string(os.PathSeparator)) && !filepath.IsAbs(back)
}
func loadPrivate(root, file string) (ed25519.PrivateKey, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	if inside(root, real) {
		return nil, fmt.Errorf("E-BLD-005: chave privada não pode estar dentro do repositório")
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("E-BLD-005: chave não é arquivo regular")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return trust.ParsePrivatePEM(data)
}
func GenerateSigningKeys(root, privatePath, publicPath string) error {
	base, err := absRoot(root)
	if err != nil {
		return err
	}
	if privatePath == "" || publicPath == "" {
		return fmt.Errorf("E-BLD-005: informe os dois caminhos")
	}
	privAbs, err := filepath.Abs(privatePath)
	if err != nil {
		return err
	}
	pubAbs, err := filepath.Abs(publicPath)
	if err != nil {
		return err
	}
	if privAbs == pubAbs {
		return fmt.Errorf("E-BLD-005: caminhos das chaves não podem ser iguais")
	}
	for _, p := range []string{privAbs, pubAbs} {
		parent, err := filepath.EvalSymlinks(filepath.Dir(p))
		if err != nil {
			return err
		}
		if inside(base, parent) {
			return fmt.Errorf("E-BLD-005: mantenha as chaves fora do repositório")
		}
		if _, err := os.Lstat(p); err == nil {
			return fmt.Errorf("E-BLD-005: arquivo já existe: %s", p)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	privatePEM, publicPEM, err := trust.GeneratePEM()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(privAbs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(privatePEM); err != nil {
		f.Close()
		os.Remove(privAbs)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(privAbs)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(privAbs)
		return err
	}
	pub, err := os.OpenFile(pubAbs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		os.Remove(privAbs)
		return err
	}
	if _, err := pub.Write(publicPEM); err != nil {
		pub.Close()
		os.Remove(pubAbs)
		os.Remove(privAbs)
		return err
	}
	if err := pub.Close(); err != nil {
		os.Remove(pubAbs)
		os.Remove(privAbs)
		return err
	}
	return nil
}
func copyFile(source, destination string) error {
	info, err := regularFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, err := io.CopyBuffer(out, in, make([]byte, 1<<20))
	if err != nil {
		out.Close()
		return err
	}
	if n != info.Size() {
		out.Close()
		return fmt.Errorf("E-BLD-003: tamanho alterado durante cópia")
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
func compileWindowsEngine(root, out, publicHex string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ldflags := "-s -w"
	if publicHex != "" {
		ldflags += " -X main.trustedCatalogPublicKeyHex=" + publicHex
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", ldflags, "-o", out, "./cmd/deploycraft")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("E-BLD-006: compilação do motor falhou: %w: %s", err, string(output))
	}
	return nil
}

// BuildFolder checks source hashes, creates a new kit, optionally signs its catalog,
// and compiles the matching Windows engine with the PUBLIC key pinned into it.
func BuildFolder(o BuildOptions) (BuildReport, error) {
	if o.SourceRoot == "" {
		return buildFolder(o, compileWindowsEngine)
	}
	sourceRoot, err := absRoot(o.SourceRoot)
	if err != nil {
		return BuildReport{}, err
	}
	return buildFolder(o, func(_ string, out, key string) error {
		return compileWindowsEngine(sourceRoot, out, key)
	})
}
func buildFolder(o BuildOptions, compiler func(root, out, key string) error) (BuildReport, error) {
	report := BuildReport{}
	root, err := absRoot(o.Root)
	if err != nil {
		return report, err
	}
	c, err := catalog.Load(root)
	if err != nil {
		return report, err
	}
	if issues := catalog.Verify(root, c); len(issues) > 0 {
		return report, fmt.Errorf("%s %s: %s", issues[0].Code, issues[0].Path, issues[0].Problem)
	}
	if o.PrivateKeyPath == "" && !o.LabUnsigned {
		return report, fmt.Errorf("E-BLD-005: assine com --private-key ou use --lab-unsigned explicitamente")
	}
	if o.PrivateKeyPath != "" && o.LabUnsigned {
		return report, fmt.Errorf("E-BLD-005: modo assinado e LAB são exclusivos")
	}
	var private ed25519.PrivateKey
	publicHex := ""
	if o.PrivateKeyPath != "" {
		private, err = loadPrivate(root, o.PrivateKeyPath)
		if err != nil {
			return report, err
		}
		publicHex = hex.EncodeToString(private.Public().(ed25519.PublicKey))
	}
	if o.Destination == "" {
		return report, fmt.Errorf("E-BLD-002: informe --out")
	}
	dest, err := filepath.Abs(o.Destination)
	if err != nil {
		return report, err
	}
	if _, err := os.Lstat(dest); err == nil {
		return report, fmt.Errorf("E-BLD-002: saída já existe: %s", dest)
	} else if !os.IsNotExist(err) {
		return report, err
	}
	if inside(filepath.Join(root, "catalog"), dest) || inside(filepath.Join(root, "payloads"), dest) {
		return report, fmt.Errorf("E-BLD-002: saída não pode ficar em catalog/ nem payloads/")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return report, err
	}
	temp, err := os.MkdirTemp(filepath.Dir(dest), ".deploycraft-build-*")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(temp) // only this uniquely created staging directory
	files := []string{"catalog/catalog.json"}
	for _, rel := range c.Index.Packages {
		files = append(files, "catalog/"+rel)
	}
	for _, p := range c.Packages {
		if p.Payload.Kind == "dir" {
			for _, f := range p.Payload.Files {
				files = append(files, p.Payload.Path+"/"+f.Path)
			}
		} else {
			files = append(files, p.Payload.Path)
		}
	}
	sort.Strings(files)
	for _, rel := range files {
		source := filepath.Join(root, filepath.FromSlash(rel))
		target := filepath.Join(temp, filepath.FromSlash(rel))
		if !inside(root, source) || !inside(temp, target) {
			return report, fmt.Errorf("E-BLD-003: caminho fora do kit: %s", rel)
		}
		if err := copyFile(source, target); err != nil {
			return report, err
		}
	}
	if issues := catalog.Verify(temp, c); len(issues) > 0 {
		return report, fmt.Errorf("%s: cópia não confere: %s", issues[0].Code, issues[0].Path)
	}
	if private != nil {
		content, err := trust.Sign(temp, private)
		if err != nil {
			return report, err
		}
		if err := os.WriteFile(filepath.Join(temp, "catalog", "catalog.sig"), content, 0600); err != nil {
			return report, err
		}
		if err := trust.Verify(temp, private.Public().(ed25519.PublicKey)); err != nil {
			return report, err
		}
	}
	engine := filepath.Join(temp, "DeployCraft.exe")
	if err := compiler(root, engine, publicHex); err != nil {
		return report, err
	}
	b, err := os.ReadFile(engine)
	if err != nil {
		return report, err
	}
	sum := sha256.Sum256(b)
	report = BuildReport{Destination: dest, CatalogVersion: c.Index.CatalogVersion, PackageCount: len(c.Packages),
		Signed: private != nil, PublicKeyHex: publicHex, EngineSHA256: hex.EncodeToString(sum[:])}
	note := fmt.Sprintf("DeployCraft offline kit\nCatalogo: %s\nPacotes: %d\nMotor SHA-256: %s\nCatalogo assinado: %t\nSem assinatura Authenticode: nao homologado para producao.\n", report.CatalogVersion, report.PackageCount, report.EngineSHA256, report.Signed)
	if err := os.WriteFile(filepath.Join(temp, "LEIAME.txt"), []byte(note), 0600); err != nil {
		return report, err
	}
	if _, err := os.Lstat(dest); err == nil {
		return report, fmt.Errorf("E-BLD-002: saída criada em paralelo; recusado")
	}
	if err := os.Rename(temp, dest); err != nil {
		return report, err
	}
	return report, nil
}
