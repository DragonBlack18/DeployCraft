// Package detect reports installed, outdated, absent or unknown.
// A read failure is never interpreted as an absent installation.
package detect

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

var winVar = regexp.MustCompile(`%[^%]+%`)

var uninstallRoots = []string{
	`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
	`HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	`HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
}

type regEntry struct {
	Name string
	Version string
	KeyName string
}
type Scan struct {
	Registry domain.Registry
	Paths domain.Paths
	entries []regEntry
	scanError error
	scanned bool
}

// NewScan creates a per-run detector. The uninstall registry is indexed once.
func NewScan(r domain.Registry, paths domain.Paths) *Scan {
	return &Scan{Registry:r, Paths:paths}
}
func (s *Scan) loadEntries() {
	if s.scanned { return }
	s.scanned = true
	for _, root := range uninstallRoots {
		keys, err := s.Registry.ListSubKeys(root)
		if err != nil {
			s.scanError = fmt.Errorf("registry %s: %w",root,err)
			continue
		}
		for _,key := range keys {
			full := root+`\`+key
			name, ok, err := s.Registry.ReadString(full,"DisplayName")
			if err != nil { s.scanError = fmt.Errorf("%s: %w",full,err);continue }
			if !ok || name=="" { continue }
			version,_,err := s.Registry.ReadString(full,"DisplayVersion")
			if err != nil { s.scanError = fmt.Errorf("%s: %w",full,err);continue }
			s.entries = append(s.entries,regEntry{Name:name,Version:version,KeyName:key})
		}
	}
}
func (s *Scan) Detect(p domain.Package) domain.Detection {
	if s.Registry == nil || s.Paths == nil {
		return domain.Detection{State:domain.StateUnknown,Evidence:[]string{"detector sem adaptadores"}}
	}
	if len(p.Detect.Rules)==0 {
		return domain.Detection{State:domain.StateUnknown,Evidence:[]string{"detect.rules vazio"}}
	}
	var results []domain.Detection
	for _,r := range p.Detect.Rules { results = append(results,s.probe(p,r)) }
	out := domain.Detection{State:domain.StateAbsent}
	unknown, outdated, installed, absent := false,false,false,false
	for _,r:=range results{
		out.Evidence=append(out.Evidence,r.Evidence...)
		switch r.State {
		case domain.StateInstalled: installed=true; if out.Version==""{out.Version=r.Version}
		case domain.StateOutdated: outdated=true; if out.Version==""{out.Version=r.Version}
		case domain.StateUnknown: unknown=true
		default: absent=true
		}
	}
	if p.Detect.Match=="any" {
		switch {
		case installed: out.State=domain.StateInstalled
		case unknown: out.State=domain.StateUnknown
		case outdated: out.State=domain.StateOutdated
		default: out.State=domain.StateAbsent
		}
		return out
	}
	switch {
	case unknown: out.State=domain.StateUnknown
	case absent: out.State=domain.StateAbsent
	case outdated: out.State=domain.StateOutdated
	case installed: out.State=domain.StateInstalled
	default: out.State=domain.StateUnknown
	}
	return out
}
func (s *Scan) probe(p domain.Package, rule domain.DetectRule) domain.Detection {
	answer:=func(state domain.State, version, message string) domain.Detection {
		return domain.Detection{State:state,Version:version,Evidence:[]string{message}}
	}
	switch rule.Type {
	case "registry-uninstall","msi-product-code":
		s.loadEntries()
		if s.scanError!=nil {return answer(domain.StateUnknown,"",s.scanError.Error())}
		for _,e:=range s.entries {
			matched:=false
			if rule.Type=="msi-product-code" {
				matched=strings.EqualFold(e.KeyName,rule.ProductCode)
			} else {
				pattern:=strings.ToLower(rule.DisplayName)
				matched,_=path.Match(pattern,strings.ToLower(e.Name))
			}
			if !matched {continue}
			min:=rule.MinVersion
			if min=="" {min=p.Version}
			if e.Version=="" {return answer(domain.StateUnknown,"","versão desconhecida: "+e.Name)}
			cmp,err:=domain.CompareVersions(e.Version,min)
			if err!=nil {return answer(domain.StateUnknown,e.Version,err.Error())}
			if cmp<0{return answer(domain.StateOutdated,e.Version,"versão antiga: "+e.Name+" "+e.Version)}
			return answer(domain.StateInstalled,e.Version,"registro confirma: "+e.Name+" "+e.Version)
		}
		return answer(domain.StateAbsent,"","nenhuma entrada de registro correspondente")
	case "file":
		file,err:=expandWindows(s.Paths,rule.Path)
		if err!=nil {return answer(domain.StateUnknown,"",err.Error())}
		info,err:=os.Stat(file)
		if os.IsNotExist(err){return answer(domain.StateAbsent,"","arquivo não existe: "+file)}
		if err!=nil{return answer(domain.StateUnknown,"",err.Error())}
		if !info.Mode().IsRegular(){return answer(domain.StateUnknown,"","caminho não é arquivo regular")}
		if rule.MinVersion!="" {
			return answer(domain.StateUnknown,"","versão do arquivo ainda não disponível: "+file)
		}
		return answer(domain.StateInstalled,"","arquivo presente: "+file)
	case "registry-value":
		value,ok,err:=s.Registry.ReadString(rule.Key,rule.Value)
		if err!=nil{return answer(domain.StateUnknown,"",err.Error())}
		if !ok {return answer(domain.StateAbsent,"","valor do registro ausente")}
		if len(rule.Equals)>0 {
			var expected any
			if err:=jsonUnmarshal(rule.Equals,&expected);err!=nil{return answer(domain.StateUnknown,"",err.Error())}
			if fmt.Sprint(expected)!=value{return answer(domain.StateAbsent,"","valor inesperado")}
		}
		if rule.AtLeast!=nil {
			actual,err:=strconv.ParseInt(value,10,64)
			if err!=nil{return answer(domain.StateUnknown,"",err.Error())}
			if actual<*rule.AtLeast{return answer(domain.StateOutdated,value,"valor abaixo do mínimo")}
		}
		if rule.MinVersion!="" {
			cmp,err:=domain.CompareVersions(value,rule.MinVersion)
			if err!=nil{return answer(domain.StateUnknown,value,err.Error())}
			if cmp<0{return answer(domain.StateOutdated,value,"versão abaixo do mínimo")}
		}
		return answer(domain.StateInstalled,value,"valor do registro confirmado")
	default:
		return answer(domain.StateUnknown,"","regra ainda não implementada: "+rule.Type)
	}
}
func expandWindows(paths domain.Paths, input string)(string,error) {
	if input=="" {return "",fmt.Errorf("caminho vazio")}
	out:=winVar.ReplaceAllStringFunc(input,func(part string)string{
		return paths.Expand(part)
	})
	if winVar.MatchString(out){return "",fmt.Errorf("variável de caminho não resolvida: %s",out)}
	// A drive path is absolute on Windows, but Linux tests use fake local paths.
	if strings.Contains(out,".."+string(filepath.Separator)) {return "",fmt.Errorf("caminho inválido")}
	return out,nil
}
