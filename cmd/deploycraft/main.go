// DeployCraft CLI: offline software deployment on Windows x64.
// The Phase 1 installer requires explicit LAB-only consent until signed kits ship.
package main

import (
	"context"
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
	"github.com/DragonBlack18/DeployCraft/src/platform"
	"github.com/DragonBlack18/DeployCraft/src/runner"
)

var version = "0.2.0-dev"

func usage() {
	fmt.Fprintln(os.Stderr, "DeployCraft",version, "— implantação offline Windows (fase 1 LAB)")
	fmt.Fprintln(os.Stderr, "Uso:")
	fmt.Fprintln(os.Stderr, "  DeployCraft version")
	fmt.Fprintln(os.Stderr, "  DeployCraft list --kit <pasta>")
	fmt.Fprintln(os.Stderr, "  DeployCraft status --kit <pasta>")
	fmt.Fprintln(os.Stderr, "  DeployCraft install --kit <pasta> --yes --lab-unsigned-catalog <id>")
	fmt.Fprintln(os.Stderr, "  DeployCraft install --kit <pasta> --lab-unsigned-catalog --dry-run <id>")
	fmt.Fprintln(os.Stderr, "O instalador ainda não possui assinatura de catálogo. Instalação apenas em VM de laboratório.")
}
func sortedIDs(c domain.Catalog)[]string{
	ids:=make([]string,0,len(c.Packages))
	for id:=range c.Packages{ids=append(ids,id)}
	sort.Strings(ids)
	return ids
}
func fail(err error) {
	fmt.Fprintln(os.Stderr,err)
	var pre *app.PreflightError
	if errors.As(err,&pre) {
		switch {
		case strings.HasPrefix(pre.Code,"E-VER"): os.Exit(domain.ExitIntegrity)
		case strings.HasPrefix(pre.Code,"E-PLN"): os.Exit(domain.ExitUsage)
		case strings.HasPrefix(pre.Code,"E-PRE"): os.Exit(domain.ExitPrecheck)
		default: os.Exit(domain.ExitPartial)
		}
	}
	os.Exit(domain.ExitPartial)
}
func main(){
	if len(os.Args)<2{usage();os.Exit(domain.ExitUsage)}
	command:=os.Args[1]
	if command=="version"{fmt.Println("DeployCraft",version);return}
	if command!="list" && command!="status" && command!="install"{
		usage();os.Exit(domain.ExitUsage)
	}
	flags:=flag.NewFlagSet(command,flag.ContinueOnError)
	kit:=flags.String("kit",".","raiz de kit")
	yes:=flags.Bool("yes",false,"confirmar execução")
	lab:=flags.Bool("lab-unsigned-catalog",false,"permitir catálogo sem assinatura apenas em laboratório")
	dry:=flags.Bool("dry-run",false,"não executar alterações")
	if err:=flags.Parse(os.Args[2:]);err!=nil{os.Exit(domain.ExitUsage)}
	c,err:=catalog.Load(*kit)
	if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(domain.ExitCatalog)}
	switch command {
	case "list":
		fmt.Printf("DeployCraft | catálogo %s | %d pacotes (sem verificação física)\n",c.Index.CatalogVersion,len(c.Packages))
		for _,id:=range sortedIDs(c){
			p:=c.Packages[id]
			fmt.Printf("  %s | %s | %s | %s\n",p.ID,p.Name,p.Version,p.Install.Driver)
		}
		profiles:=make([]string,0,len(c.Profiles))
		for id:=range c.Profiles{profiles=append(profiles,id)}
		sort.Strings(profiles)
		fmt.Printf("Perfis (%d):\n",len(profiles))
		for _,id:=range profiles{fmt.Println("  ",id)}
	case "status":
		if runtime.GOOS!="windows"{fail(fmt.Errorf("E-PRE-002: status real exige Windows"))}
		scan:=detect.NewScan(platform.NewRegistry(),platform.NewPaths())
		for _,id:=range sortedIDs(c) {
			p:=c.Packages[id]
			s:=scan.Detect(p)
			fmt.Printf("%s | %s | detectado=%s | versão=%s\n",id,p.Name,s.State,s.Version)
			for _,line:=range s.Evidence{fmt.Println("   ",line)}
		}
	case "install":
		if len(flags.Args())!=1{usage();os.Exit(domain.ExitUsage)}
		if runtime.GOOS!="windows"{fail(fmt.Errorf("E-PRE-002: instale somente em Windows"))}
		if *kit=="." {fmt.Fprintln(os.Stderr,"Aviso: usando kit a partir da pasta atual")}
		unlock,err:=platform.AcquireInstallerLock()
		if err!=nil{fail(err)}
		defer unlock()
		programData:=platform.NewPaths().ProgramData()
		if programData==""{fail(fmt.Errorf("E-PRE-003: ProgramData indisponível"))}
		absoluteKit,err:=filepath.Abs(*kit)
		if err!=nil{fail(err)}
		engine:=&app.InstallEngine{
			Catalog:c,KitRoot:absoluteKit,StateRoot:filepath.Join(programData,"DeployCraft"),
			Registry:platform.NewRegistry(),Paths:platform.NewPaths(),Process:runner.New(),
			IsAdmin:platform.IsAdministrator,WindowsBuild:platform.WindowsBuild,
		}
		report,err:=engine.InstallOne(context.Background(),flags.Arg(0),app.InstallOptions{
			LabUnsignedCatalog:*lab,Confirmed:*yes,DryRun:*dry,
		})
		if err!=nil{fail(err)}
		for _,r:=range report.Results{
			fmt.Printf("%s | %s | %s\n",r.PackageID,r.Outcome,r.Detail)
			if r.Code!=""{fmt.Printf("  erro: %s (%s)\n",r.Code,r.Reason)}
		}
		if report.RunID!=""{fmt.Println("Run ID:",report.RunID)}
		if report.ExitCode!=0{os.Exit(report.ExitCode)}
	}
}
