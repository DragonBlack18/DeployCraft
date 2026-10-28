// kit-builder operates on offline files on the build computer, never on the
// destination Windows workstation. Private keys are never stored in the kit.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/DragonBlack18/DeployCraft/src/catalog"
	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/kitbuilder"
	"github.com/DragonBlack18/DeployCraft/src/trust"
)

type multiFlag []string

func (m *multiFlag) String() string     { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(s string) error { *m = append(*m, s); return nil }
func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
func usage() {
	fmt.Fprintln(os.Stderr, "DeployCraft kit-builder — comandos:")
	fmt.Fprintln(os.Stderr, "  lint|verify [--root .] [--public-key caminho]")
	fmt.Fprintln(os.Stderr, "  add --root . --file arquivo.exe --id id --name Nome --version 1.0 --driver exe --arg /S --detect-name 'Nome*'")
	fmt.Fprintln(os.Stderr, "  update --root . --id id --version 1.1 --file arquivo.exe")
	fmt.Fprintln(os.Stderr, "  keygen --root . --private caminho-fora-do-repo --public caminho-fora-do-repo")
	fmt.Fprintln(os.Stderr, "  build --mode folder --root . --out dist/kit-1 --private-key caminho-fora-do-repo")
	fmt.Fprintln(os.Stderr, "  build --mode folder --root . --out dist/kit-lab --lab-unsigned")
	fmt.Fprintln(os.Stderr, "  report --root .")
	fmt.Fprintln(os.Stderr, "  SFX não implementado nesta etapa; jamais usar binários de terceiros no Git.")
}
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(domain.ExitUsage)
	}
	command := os.Args[1]
	switch command {
	case "lint", "verify", "report":
		f := flag.NewFlagSet(command, flag.ExitOnError)
		root := f.String("root", ".", "pasta do repositório ou kit")
		public := f.String("public-key", "", "chave pública PEM para validar catalog.sig")
		_ = f.Parse(os.Args[2:])
		c, err := catalog.Load(*root)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Catálogo válido: %s, pacotes=%d, perfis=%d\n", c.Index.CatalogVersion, len(c.Packages), len(c.Profiles))
		if command == "lint" {
			fmt.Println("Lint: PASS")
			return
		}
		if command == "report" {
			ids := make([]string, 0, len(c.Packages))
			for id := range c.Packages {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			var total int64
			for _, id := range ids {
				p := c.Packages[id]
				var bytes int64
				if p.Payload.Kind == "dir" {
					for _, f := range p.Payload.Files {
						bytes += f.Size
					}
				} else {
					bytes = p.Payload.Size
				}
				total += bytes
				fmt.Printf("  %s | %s | %s | %.2f MiB\n", id, p.Version, p.Install.Driver, float64(bytes)/(1<<20))
			}
			fmt.Printf("Payloads referenciados: %.2f MiB\n", float64(total)/(1<<20))
			return
		}
		issues := catalog.Verify(*root, c)
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "%s %s: %s\n", issue.Code, issue.Path, issue.Problem)
		}
		if len(issues) > 0 {
			os.Exit(domain.ExitIntegrity)
		}
		if *public != "" {
			bytes, err := os.ReadFile(*public)
			if err != nil {
				fatal(err)
			}
			key, err := trust.ParsePublicPEM(bytes)
			if err != nil {
				fatal(err)
			}
			if err := trust.Verify(*root, key); err != nil {
				fatal(err)
			}
			fmt.Println("Assinatura Ed25519: PASS")
		} else {
			fmt.Println("AVISO: hashes conferidos, assinatura do catálogo NÃO foi autenticada.")
		}
		fmt.Println("Integridade dos payloads: PASS")
	case "add":
		f := flag.NewFlagSet("add", flag.ExitOnError)
		var args, profiles multiFlag
		root := f.String("root", ".", "raiz do repositório")
		file := f.String("file", "", "instalador offline MSI ou EXE")
		id := f.String("id", "", "identificador em kebab-case")
		name := f.String("name", "", "nome para exibição")
		version := f.String("version", "", "versão numérica")
		driver := f.String("driver", "", "msi ou exe")
		detectName := f.String("detect-name", "", "nome no Registro (aceita wildcard)")
		detectFile := f.String("detect-file", "", "caminho do programa após instalação")
		source := f.String("source-url", "", "origem oficial para auditoria")
		vendorHash := f.String("vendor-hash", "", "SHA-256 publicado pelo fabricante")
		f.Var(&args, "arg", "switch de instalação (repetir)")
		f.Var(&profiles, "profile", "adicionar ao perfil existente (repetir)")
		_ = f.Parse(os.Args[2:])
		p, err := kitbuilder.Add(kitbuilder.ImportOptions{
			Root: *root, Input: *file, ID: *id, Name: *name, Version: *version, Driver: *driver,
			Args: args, DetectName: *detectName, DetectFile: *detectFile,
			SourceURL: *source, VendorHash: *vendorHash, Profiles: profiles,
		})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Adicionado: %s %s, SHA-256 %s\n", p.ID, p.Version, p.Payload.SHA256)
		fmt.Println("Pendente: conferir switches, assinatura do fornecedor e detecção em VM Windows.")
	case "update":
		f := flag.NewFlagSet("update", flag.ExitOnError)
		root := f.String("root", ".", "raiz do repositório")
		id := f.String("id", "", "identificador do pacote")
		version := f.String("version", "", "nova versão numérica")
		file := f.String("file", "", "novo MSI/EXE offline")
		source := f.String("source-url", "", "origem oficial")
		vendorHash := f.String("vendor-hash", "", "hash publicado")
		_ = f.Parse(os.Args[2:])
		p, err := kitbuilder.Update(kitbuilder.UpdateOptions{Root: *root, ID: *id, Input: *file, Version: *version, SourceURL: *source, VendorHash: *vendorHash})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Atualizado: %s %s, SHA-256 %s\n", p.ID, p.Version, p.Payload.SHA256)
		fmt.Println("O payload anterior não foi apagado. Revalide os switches e a detecção.")
	case "keygen":
		f := flag.NewFlagSet("keygen", flag.ExitOnError)
		root := f.String("root", ".", "repositório usado para checagem de segurança")
		priv := f.String("private", "", "caminho da chave privada FORA do repositório")
		pub := f.String("public", "", "caminho da chave pública FORA do repositório")
		_ = f.Parse(os.Args[2:])
		if err := kitbuilder.GenerateSigningKeys(*root, *priv, *pub); err != nil {
			fatal(err)
		}
		encoded, err := os.ReadFile(*pub)
		if err != nil {
			fatal(err)
		}
		key, err := trust.ParsePublicPEM(encoded)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Par de chaves criado. ID público: %s; chave hex: %s\n", trust.KeyID(key), hex.EncodeToString(key))
		fmt.Println("Guarde a chave privada em local restrito; ela NÃO vai ao Git nem ao kit.")
	case "build":
		f := flag.NewFlagSet("build", flag.ExitOnError)
		root := f.String("root", ".", "raiz do repositório")
		out := f.String("out", "", "novo destino (não deve existir)")
		mode := f.String("mode", "folder", "folder; sfx planejado para fase posterior")
		key := f.String("private-key", "", "chave privada Ed25519 fora do Git")
		lab := f.Bool("lab-unsigned", false, "kit de laboratório explicitamente não assinado")
		_ = f.Parse(os.Args[2:])
		if *mode != "folder" {
			fatal(fmt.Errorf("E-BLD-007: modo %q não implementado; não será gerado arquivo SFX inválido", *mode))
		}
		report, err := kitbuilder.BuildFolder(kitbuilder.BuildOptions{Root: *root, Destination: *out, PrivateKeyPath: *key, LabUnsigned: *lab})
		if err != nil {
			fatal(err)
		}
		fmt.Printf("Kit gerado: %s | catálogo %s | pacotes %d | assinatura %v | motor SHA-256 %s\n",
			report.Destination, report.CatalogVersion, report.PackageCount, report.Signed, report.EngineSHA256)
		if !report.Signed {
			fmt.Println("LAB: catálogo não assinado, NÃO utilizar em produção.")
		}
		fmt.Println("Nota: Authenticode do executável e homologação em VM ainda pendentes.")
	default:
		usage()
		os.Exit(domain.ExitUsage)
	}
}
