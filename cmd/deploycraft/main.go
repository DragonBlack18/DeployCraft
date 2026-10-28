// DeployCraft CLI — universal offline deployment for Windows 10/11 x64.
// Until catalog signatures are implemented, installs are LAB-ONLY.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/app"
	"github.com/DragonBlack18/DeployCraft/src/catalog"
	"github.com/DragonBlack18/DeployCraft/src/detect"
	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/planner"
	"github.com/DragonBlack18/DeployCraft/src/platform"
	"github.com/DragonBlack18/DeployCraft/src/runner"
)

var version = "0.3.2-sfx-lab"

// A SFX build pins "1"; a broken package then fails closed instead of
// silently treating the executable's directory as a folder kit.
var sfxExpected string

// Set at build time with -ldflags "-X main.trustedCatalogPublicKeyHex=<hex>".
// An empty value only allows explicitly confirmed, unsigned LAB mode.
var trustedCatalogPublicKeyHex string

func pinnedPublicKey() (ed25519.PublicKey, error) {
	if trustedCatalogPublicKeyHex == "" {
		return nil, nil
	}
	key, err := hex.DecodeString(trustedCatalogPublicKeyHex)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("E-VER-004: invalid public key pinned in DeployCraft.exe")
	}
	return ed25519.PublicKey(key), nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "DeployCraft", version, "— offline Windows installer (Fase 2 LAB)")
	fmt.Fprintln(os.Stderr, "  DeployCraft version")
	fmt.Fprintln(os.Stderr, "  DeployCraft list --kit <pasta>")
	fmt.Fprintln(os.Stderr, "  DeployCraft status --kit <pasta>")
	fmt.Fprintln(os.Stderr, "  DeployCraft plan --kit <pasta> --profile desenvolvimento [ids...] [--json]")
	fmt.Fprintln(os.Stderr, "  DeployCraft install --kit <pasta> --profile desenvolvimento --yes --lab-unsigned-catalog")
	fmt.Fprintln(os.Stderr, "  DeployCraft install --kit <pasta> --dry-run --profile desenvolvimento")
	fmt.Fprintln(os.Stderr, "  DeployCraft resume --kit <pasta> --yes --lab-unsigned-catalog <runId>")
	fmt.Fprintln(os.Stderr, "Instalação e retomada APENAS em VM descartável até a assinatura do catálogo.")
}
func sortedIDs(c domain.Catalog) []string {
	ids := make([]string, 0, len(c.Packages))
	for id := range c.Packages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	var pre *app.PreflightError
	if errors.As(err, &pre) {
		switch {
		case strings.HasPrefix(pre.Code, "E-VER"):
			os.Exit(domain.ExitIntegrity)
		case strings.HasPrefix(pre.Code, "E-PLN"):
			os.Exit(domain.ExitUsage)
		case strings.HasPrefix(pre.Code, "E-RES-002"), strings.HasPrefix(pre.Code, "E-PRE"):
			os.Exit(domain.ExitPrecheck)
		default:
			os.Exit(domain.ExitPartial)
		}
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "E-VER"):
		os.Exit(domain.ExitIntegrity)
	case strings.HasPrefix(msg, "E-PLN"), strings.HasPrefix(msg, "E-RES"):
		os.Exit(domain.ExitUsage)
	default:
		os.Exit(domain.ExitPartial)
	}
}
func printPlan(plan domain.Plan, jsonMode bool) {
	if jsonMode {
		b, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			exitErr(err)
		}
		fmt.Println(string(b))
		return
	}
	if len(plan.Phases) == 0 {
		fmt.Println("Nenhum pacote precisa ser processado.")
		return
	}
	for i, phase := range plan.Phases {
		fmt.Printf("Fase %d:\n", i+1)
		for _, step := range phase.Steps {
			fmt.Printf("  %s | %s | %s\n", step.PackageID, step.Action, step.Reason)
		}
		if phase.RebootAfter {
			fmt.Println("  [barreira de reinício, se solicitada pelo instalador]")
		}
	}
}
func printReport(report app.PackageReport, jsonMode bool) {
	if jsonMode {
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			exitErr(err)
		}
		fmt.Println(string(b))
		return
	}
	fmt.Println("Execução:", report.RunID)
	for _, r := range report.Results {
		fmt.Printf("  %s | %s | %s\n", r.PackageID, r.Outcome, r.Detail)
		if r.Code != "" {
			fmt.Printf("    %s (%s)\n", r.Code, r.Reason)
		}
	}
	fmt.Printf("Código final: %d\n", report.ExitCode)
}
func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(domain.ExitUsage)
	}
	command := os.Args[1]
	if command == "version" {
		fmt.Println("DeployCraft", version)
		return
	}
	switch command {
	case "list", "status", "plan", "install", "resume":
	default:
		usage()
		os.Exit(domain.ExitUsage)
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	kit := flags.String("kit", ".", "raiz do kit")
	profile := flags.String("profile", "", "perfil do catálogo")
	yes := flags.Bool("yes", false, "confirmar execução")
	lab := flags.Bool("lab-unsigned-catalog", false, "somente VM LAB, catálogo sem assinatura")
	dry := flags.Bool("dry-run", false, "calcular plano sem executar")
	force := flags.Bool("force", false, "reinstalar mesmo se instalado")
	stop := flags.Bool("stop-on-error", false, "parar na primeira falha")
	jsonMode := flags.Bool("json", false, "formato de saída JSON")
	if err := flags.Parse(os.Args[2:]); err != nil {
		os.Exit(domain.ExitUsage)
	}
	if command == "resume" && (*profile != "" || *force || *dry || len(flags.Args()) != 1) {
		usage()
		os.Exit(domain.ExitUsage)
	}
	if (command == "install" || command == "plan") && *profile == "" && len(flags.Args()) == 0 {
		usage()
		os.Exit(domain.ExitUsage)
	}
	var payloadSource domain.PayloadSource
	if sfxExpected == "1" {
		if *kit != "." {
			exitErr(fmt.Errorf("E-SFX-001: kit externo não é permitido com SFX"))
		}
		embeddedRoot, reader, err := openEmbeddedKit()
		if err != nil {
			exitErr(err)
		}
		defer reader.Close()
		*kit = embeddedRoot
		payloadSource = reader
	}
	c, err := catalog.Load(*kit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(domain.ExitCatalog)
	}
	if command == "list" {
		fmt.Printf("DeployCraft | catálogo %s | %d pacotes\n", c.Index.CatalogVersion, len(c.Packages))
		for _, id := range sortedIDs(c) {
			p := c.Packages[id]
			fmt.Printf("  %s | %s | %s | %s\n", p.ID, p.Name, p.Version, p.Install.Driver)
		}
		profiles := make([]string, 0, len(c.Profiles))
		for id := range c.Profiles {
			profiles = append(profiles, id)
		}
		sort.Strings(profiles)
		fmt.Printf("Perfis (%d):\n", len(profiles))
		for _, id := range profiles {
			fmt.Println("  ", id)
		}
		return
	}
	if runtime.GOOS != "windows" {
		exitErr(fmt.Errorf("E-PRE-002: detecção e execução reais exigem Windows"))
	}
	if command == "status" {
		scan := detect.NewScan(platform.NewRegistry(), platform.NewPaths())
		for _, id := range sortedIDs(c) {
			p := c.Packages[id]
			status := scan.Detect(p)
			fmt.Printf("%s | %s | estado=%s | versão=%s\n", id, p.Name, status.State, status.Version)
			for _, detail := range status.Evidence {
				fmt.Println("   ", detail)
			}
		}
		return
	}
	absoluteKit, err := filepath.Abs(*kit)
	if err != nil {
		exitErr(err)
	}
	dataRoot := platform.NewPaths().ProgramData()
	if dataRoot == "" {
		exitErr(&app.PreflightError{Code: "E-PRE-003", Reason: "ProgramData indisponível"})
	}
	trustedKey, err := pinnedPublicKey()
	if err != nil {
		exitErr(err)
	}
	engine := &app.InstallEngine{
		Catalog: c, KitRoot: absoluteKit, StateRoot: filepath.Join(dataRoot, "DeployCraft"),
		Registry: platform.NewRegistry(), Paths: platform.NewPaths(), Process: runner.New(),
		IsAdmin: platform.IsAdministrator, WindowsBuild: platform.WindowsBuild,
		TrustedCatalogPublicKey: trustedKey,
		PayloadSource: payloadSource,
	}
	selection := planner.Selection{Profile: *profile, IDs: flags.Args()}
	planOptions := planner.Options{Force: *force, UpgradeOutdated: true}
	if command == "plan" || (command == "install" && *dry) {
		plan, err := engine.PlanSelection(selection, planOptions)
		if err != nil {
			exitErr(err)
		}
		printPlan(plan, *jsonMode)
		return
	}
	unlock, err := platform.AcquireInstallerLock()
	if err != nil {
		exitErr(err)
	}
	defer unlock()
	resumeID := ""
	if command == "resume" {
		resumeID = flags.Arg(0)
		selection = planner.Selection{}
	}
	report, err := engine.RunBatch(context.Background(), selection, app.BatchOptions{
		InstallOptions: app.InstallOptions{LabUnsignedCatalog: *lab, Confirmed: *yes},
		Force:          *force, StopOnError: *stop,
	}, resumeID)
	if err != nil {
		exitErr(err)
	}
	printReport(report, *jsonMode)
	if report.ExitCode == domain.ExitReboot {
		fmt.Fprintf(os.Stderr, "Reinício pendente. Havendo fases restantes, reinicie e use: DeployCraft resume --kit %q --yes --lab-unsigned-catalog %s\n", absoluteKit, report.RunID)
	}
	if report.ExitCode != domain.ExitOK {
		os.Exit(report.ExitCode)
	}
}
