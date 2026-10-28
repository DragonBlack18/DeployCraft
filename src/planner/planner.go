// Package planner converts catalog, selection and detection snapshot into a deterministic,
// read-only installation plan. This package must never touch the OS.
package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

// Selection requests one profile and/or explicit package IDs.
type Selection struct {
	Profile string `json:"profile,omitempty"`
	IDs []string `json:"ids,omitempty"`
}

// Options cannot authorize OS mutations. Force is a planning policy only.
type Options struct {
	Force bool `json:"force,omitempty"`
	UpgradeOutdated bool `json:"upgradeOutdated"`
}

// Build produces topologically ordered phases and refuses unknown detection states.
func Build(c domain.Catalog, selection Selection, states map[string]domain.Detection, opt Options) (domain.Plan,error) {
	plan:=domain.Plan{Phases:[]domain.Phase{}}
	if selection.Profile=="" && len(selection.IDs)==0 {
		return plan,fmt.Errorf("E-PLN-001: selecione um perfil ou pacote")
	}
	chosen:=map[string]bool{}
	explicit:=map[string]bool{}
	for _,id:=range selection.IDs {
		if _,ok:=c.Packages[id];!ok{return plan,fmt.Errorf("E-PLN-001: pacote não encontrado: %s",id)}
		chosen[id]=true
		explicit[id]=true
	}
	if selection.Profile!="" {
		visited:=map[string]bool{}
		var expand func(string)error
		expand=func(id string)error{
			if visited[id]{return fmt.Errorf("E-PLN-007: ciclo de perfis: %s",id)}
			p,ok:=c.Profiles[id]
			if !ok{return fmt.Errorf("E-PLN-001: perfil desconhecido: %s",id)}
			visited[id]=true
			if p.Extends!=""{if err:=expand(p.Extends);err!=nil{return err}}
			visited[id]=false
			for _,pkgID:=range p.Packages{
				if _,ok:=c.Packages[pkgID];!ok{return fmt.Errorf("E-PLN-001: perfil %s referencia %s inexistente",id,pkgID)}
				chosen[pkgID]=true
				explicit[pkgID]=true
			}
			return nil
		}
		if err:=expand(selection.Profile);err!=nil{return plan,err}
	}
	roots:=keys(chosen)
	dependsOn:=map[string][]string{}
	visiting:=map[string]bool{}
	complete:=map[string]bool{}
	var addDependencies func(string)error
	addDependencies=func(id string)error{
		if visiting[id]{return fmt.Errorf("E-PLN-007: ciclo de dependências: %s",id)}
		if complete[id]{return nil}
		p,ok:=c.Packages[id]
		if !ok{return fmt.Errorf("E-PLN-001: dependência desconhecida: %s",id)}
		visiting[id]=true
		deps:=append([]string(nil),p.DependsOn...)
		sort.Strings(deps)
		for _,d:=range deps{
			if d==id{return fmt.Errorf("E-PLN-007: autodependência: %s",id)}
			if _,ok:=c.Packages[d];!ok{return fmt.Errorf("E-PLN-001: %s depende de pacote inexistente %s",id,d)}
			chosen[d]=true
			if err:=addDependencies(d);err!=nil{return err}
		}
		dependsOn[id]=deps
		visiting[id]=false
		complete[id]=true
		return nil
	}
	for _,id:=range roots {if err:=addDependencies(id);err!=nil{return plan,err}}
	orderedIDs:=keys(chosen)
	for _,id:=range orderedIDs{
		got,ok:=states[id]
		if !ok || got.State==domain.StateUnknown || got.State==""{
			return plan,fmt.Errorf("E-PLN-004: detecção indeterminada de %s",id)
		}
		switch got.State{
		case domain.StateAbsent,domain.StateInstalled,domain.StateOutdated:
		default:return plan,fmt.Errorf("E-PLN-004: estado inválido de %s: %q",id,got.State)
		}
	}
	// A conflict must be checked in either direction and against installed
	// packages outside the selection. Missing evidence blocks the operation.
	for _,id:=range orderedIDs{
		p:=c.Packages[id]
		for _,other:=range p.ConflictsWith{
			if other==id{return plan,fmt.Errorf("E-PLN-002: conflito próprio %s",id)}
			if chosen[other]{return plan,fmt.Errorf("E-PLN-002: pacotes incompatíveis: %s e %s",id,other)}
			detected,ok:=states[other]
			if !ok || detected.State==domain.StateUnknown{
				return plan,fmt.Errorf("E-PLN-004: conflito %s com %s não pôde ser determinado",id,other)
			}
			if detected.State==domain.StateInstalled || detected.State==domain.StateOutdated{
				return plan,fmt.Errorf("E-PLN-002: %s conflita com %s presente",id,other)
			}
		}
		for otherID,otherP:=range c.Packages{
			if otherID==id || chosen[otherID] || !contains(otherP.ConflictsWith,id){continue}
			detected,ok:=states[otherID]
			if !ok || detected.State==domain.StateUnknown{
				return plan,fmt.Errorf("E-PLN-004: conflito reverso com %s não pôde ser determinado",otherID)
			}
			if detected.State==domain.StateInstalled || detected.State==domain.StateOutdated{
				return plan,fmt.Errorf("E-PLN-002: %s instalado conflita com %s",otherID,id)
			}
		}
	}
	outgoing:=map[string][]string{}
	degree:=map[string]int{}
	for _,id:=range orderedIDs{degree[id]=0}
	for _,id:=range orderedIDs{
		for _,d:=range dependsOn[id]{
			outgoing[d]=append(outgoing[d],id)
			degree[id]++
		}
	}
	ready:=[]string{}
	for _,id:=range orderedIDs{if degree[id]==0{ready=append(ready,id)}}
	result:=[]string{}
	for len(ready)>0{
		id:=ready[0]
		ready=ready[1:]
		result=append(result,id)
		for _,next:=range outgoing[id]{
			degree[next]--
			if degree[next]==0{ready=append(ready,next)}
		}
		sort.Strings(ready)
	}
	if len(result)!=len(orderedIDs){return plan,fmt.Errorf("E-PLN-007: dependências contêm ciclo")}
	phase:=domain.Phase{Steps:[]domain.Step{}}
	for _,id:=range result{
		detected:=states[id]
		step:=domain.Step{PackageID:id}
		switch{
		case opt.Force:
			step.Action=domain.ActionInstall
			step.Reason="reinstalação explicitamente solicitada"
		case detected.State==domain.StateInstalled:
			step.Action=domain.ActionSkip
			step.Reason="já instalado, versão "+detected.Version
		case detected.State==domain.StateOutdated:
			if !opt.UpgradeOutdated{
				return plan,fmt.Errorf("E-PLN-005: %s está desatualizado e atualização está desativada",id)
			}
			step.Action=domain.ActionUpdate
			step.Reason="versão instalada "+detected.Version+" inferior ao catálogo"
		default:
			step.Action=domain.ActionInstall
			if explicit[id]{step.Reason="selecionado pelo operador"}else{step.Reason="dependência requerida pelo perfil ou pacote"}
		}
		if strings.TrimSpace(step.Reason)==""{return plan,fmt.Errorf("E-PLN-004: motivo vazio para %s",id)}
		phase.Steps=append(phase.Steps,step)
		if c.Packages[id].Install.RebootBarrier && step.Action!=domain.ActionSkip{
			phase.RebootAfter=true
			plan.Phases=append(plan.Phases,phase)
			phase=domain.Phase{Steps:[]domain.Step{}}
		}
	}
	if len(phase.Steps)>0{plan.Phases=append(plan.Phases,phase)}
	return plan,nil
}
func keys(m map[string]bool)[]string{
	result:=make([]string,0,len(m))
	for x:=range m{result=append(result,x)}
	sort.Strings(result)
	return result
}
func contains(items []string,s string)bool{
	for _,x:=range items{if x==s{return true}}
	return false
}
