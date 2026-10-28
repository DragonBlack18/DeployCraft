// Package drivers maps validated package manifests to executable commands.
// It never starts a process. The runner owns process creation and timeouts.
package drivers

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

var msiProperty=regexp.MustCompile(`^[A-Z][A-Z0-9_]*=.+$`)
var secretMarker=regexp.MustCompile(`\{secret\}`)
var tokenMarker=regexp.MustCompile(`\{(arquivo|log|dir|runId)\}`)
var unknownMarker=regexp.MustCompile(`\{[^{}]+\}`)

type driver struct{kind string}

func New(kind string)(domain.Driver,error) {
	switch kind {
	case "msi","exe": return driver{kind:kind},nil
	default: return nil,fmt.Errorf("E-DRV-002: driver %q indisponível na Fase 1",kind)
	}
}

func (d driver) Prepare(p domain.Package, file string, ctx domain.RunContext)(domain.Command,error){
	if p.Install.Driver!=d.kind{return domain.Command{},fmt.Errorf("E-DRV-002: driver de pacote incompatível")}
	if file=="" || !filepath.IsAbs(file) {return domain.Command{},fmt.Errorf("E-DRV-002: payload não está no staging absoluto")}
	if p.Install.RawCmdLine!="" {
		return domain.Command{},fmt.Errorf("E-DRV-002: rawCmdLine exige implementação especializada (fase 4)")
	}
	base:=filepath.Dir(file)
	realArgs,logArgs:=[]string{},[]string{}
	for _,raw:=range p.Install.Args {
		var value string
		var logValue string
		if err:=json.Unmarshal(raw,&value);err==nil{
			if secretMarker.MatchString(value){return domain.Command{},fmt.Errorf("E-CAT-011: {secret} sem objeto")}
			logValue=value
		}else{
			var a struct {
				Value string `json:"value"`
				Secret string `json:"secret"`
			}
			if err:=json.Unmarshal(raw,&a);err!=nil || a.Secret=="" || !strings.Contains(a.Value,"{secret}") {
				return domain.Command{},fmt.Errorf("E-CAT-011: argumento secret inválido")
			}
			secret,ok:=ctx.Secrets[a.Secret]
			if !ok || secret=="" {return domain.Command{},fmt.Errorf("E-PLN-003: segredo %s ausente",a.Secret)}
			value=strings.ReplaceAll(a.Value,"{secret}",secret)
			logValue=strings.ReplaceAll(a.Value,"{secret}","***")
		}
		value,err:=substitute(value,file,ctx)
		if err!=nil{return domain.Command{},err}
		logValue,err=substitute(logValue,file,ctx)
		if err!=nil{return domain.Command{},err}
		if d.kind=="msi" && !msiProperty.MatchString(value) {
			return domain.Command{},fmt.Errorf("E-CAT-003: propriedade MSI inválida")
		}
		realArgs=append(realArgs,value)
		logArgs=append(logArgs,logValue)
	}
	cmd:=domain.Command{Cwd:base}
	if d.kind=="msi" {
		if strings.ToLower(filepath.Ext(file))!=".msi"{return domain.Command{},fmt.Errorf("E-DRV-002: driver MSI exige .msi")}
		if ctx.LogPath==""{return domain.Command{},fmt.Errorf("E-DRV-002: log MSI obrigatório")}
		cmd.Exe="msiexec.exe"
		cmd.Args=append([]string{"/i",file,"/qn","/norestart","/L*v",ctx.LogPath},realArgs...)
		cmd.LogLine="msiexec.exe /i "+strconv.Quote(file)+" /qn /norestart /L*v "+strconv.Quote(ctx.LogPath)
	} else {
		cmd.Exe=file
		cmd.Args=realArgs
		cmd.LogLine=strconv.Quote(file)
	}
	for _,arg:=range logArgs {cmd.LogLine+=" "+strconv.Quote(arg)}
	return cmd,nil
}
func substitute(value,file string,ctx domain.RunContext)(string,error){
	values:=map[string]string{
		"arquivo":file,
		"log":ctx.LogPath,
		"dir":filepath.Dir(file),
		"runId":ctx.RunID,
	}
	out:=tokenMarker.ReplaceAllStringFunc(value,func(t string)string{
		return values[strings.Trim(t,"{}")]
	})
	if unknownMarker.MatchString(out){return "",fmt.Errorf("E-CAT-010: marcador desconhecido")}
	return out,nil
}
func (d driver) Interpret(code int,p domain.Package)(domain.Verdict,domain.FailReason){
	for _,x:=range p.Install.RebootCodes{if code==x{return domain.VerdictSuccessReboot,""}}
	for _,x:=range p.Install.RetryCodes{if code==x{return domain.VerdictRetry,""}}
	for _,x:=range p.Install.SuccessCodes{if code==x{return domain.VerdictSuccess,""}}
	return domain.VerdictFailure,domain.ReasonExitCode
}

// SafeError prevents wrapping secret-containing arguments into errors.
func SafeError(err error) error {
	if err==nil{return nil}
	return errors.New("E-DRV-002: falha ao preparar comando; verifique o manifesto")
}
