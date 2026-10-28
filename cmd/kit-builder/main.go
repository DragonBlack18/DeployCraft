// kit-builder is the offline catalog validation tool. Import/build comes later.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/DragonBlack18/DeployCraft/src/catalog"
	"github.com/DragonBlack18/DeployCraft/src/domain"
)

func usage() {
	fmt.Fprintln(os.Stderr, "Uso: kit-builder lint|verify [--root <pasta>]")
}
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(domain.ExitUsage)
	}
	cmd := os.Args[1]
	if cmd != "lint" && cmd != "verify" {
		usage()
		os.Exit(domain.ExitUsage)
	}
	f := flag.NewFlagSet(cmd, flag.ContinueOnError)
	root := f.String("root", ".", "raiz do repositório ou kit")
	if err := f.Parse(os.Args[2:]); err != nil {
		os.Exit(domain.ExitUsage)
	}
	c, err := catalog.Load(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(domain.ExitCatalog)
	}
	fmt.Printf("Catálogo válido: %s, pacotes=%d, perfis=%d\n", c.Index.CatalogVersion, len(c.Packages), len(c.Profiles))
	if cmd == "verify" {
		issues := catalog.Verify(*root, c)
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "%s %s (%s): %s\n", issue.Code, issue.PackageID, issue.Path, issue.Problem)
		}
		if len(issues) > 0 {
			os.Exit(domain.ExitIntegrity)
		}
		fmt.Println("Integridade dos payloads verificada (SHA-256).")
	} else {
		fmt.Println("Lint estrutural concluído; execute verify para conferir payloads físicos.")
	}
}
