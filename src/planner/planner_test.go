package planner

import (
 "encoding/json"
 "math/rand"
 "reflect"
 "strings"
 "testing"
 "github.com/DragonBlack18/DeployCraft/src/domain"
)
func fixtures()(domain.Catalog,map[string]domain.Detection){
 c:=domain.Catalog{Packages:map[string]domain.Package{
 "base":{ID:"base"},
 "browser":{ID:"browser",DependsOn:[]string{"base"}},
 "editor":{ID:"editor",DependsOn:[]string{"browser","base"}},
 "git":{ID:"git"},
 },Profiles:map[string]domain.Profile{
 "essencial":{ID:"essencial",Packages:[]string{"browser"}},
 "dev":{ID:"dev",Extends:"essencial",Packages:[]string{"editor","git"}},
 }}
 d:=map[string]domain.Detection{}
 for id:=range c.Packages{d[id]=domain.Detection{State:domain.StateAbsent}}
 return c,d
}
func flatten(p domain.Plan)[]domain.Step{
 out:=[]domain.Step{}
 for _,ph:=range p.Phases{out=append(out,ph.Steps...)}
 return out
}
func TestProfileInheritanceDependenciesAndStableOrder(t *testing.T){
 c,d:=fixtures()
 p,err:=Build(c,Selection{Profile:"dev",IDs:[]string{"editor"}},d,Options{UpgradeOutdated:true})
 if err!=nil{t.Fatal(err)}
 got:=[]string{}
 for _,s:=range flatten(p){got=append(got,s.PackageID)}
 want:=[]string{"base","browser","editor","git"}
 if !reflect.DeepEqual(got,want){t.Fatalf("got %v want %v",got,want)}
 for i:=0;i<10;i++{
  q,err:=Build(c,Selection{Profile:"dev",IDs:[]string{"editor"}},d,Options{UpgradeOutdated:true})
  if err!=nil||!reflect.DeepEqual(q,p){t.Fatal("nondeterministic plan",err)}
 }
}
func TestSkipUpdateForceAndUnknown(t *testing.T){
 c,d:=fixtures()
 d["base"]=domain.Detection{State:domain.StateInstalled,Version:"4.2"}
 d["browser"]=domain.Detection{State:domain.StateOutdated,Version:"0.8"}
 p,err:=Build(c,Selection{IDs:[]string{"browser"}},d,Options{UpgradeOutdated:true})
 if err!=nil{t.Fatal(err)}
 steps:=flatten(p)
 if steps[0].Action!=domain.ActionSkip||steps[1].Action!=domain.ActionUpdate{t.Fatalf("wrong actions %+v",steps)}
 if _,err:=Build(c,Selection{IDs:[]string{"browser"}},d,Options{});err==nil{t.Fatal("outdated state ignored")}
 p,err=Build(c,Selection{IDs:[]string{"browser"}},d,Options{Force:true,UpgradeOutdated:true})
 if err!=nil{t.Fatal(err)}
 for _,s:=range flatten(p){if s.Action!=domain.ActionInstall{t.Fatalf("force ignored %+v",s)}}
 d["base"]=domain.Detection{State:domain.StateUnknown}
 if _,err:=Build(c,Selection{IDs:[]string{"browser"}},d,Options{UpgradeOutdated:true});err==nil||!strings.Contains(err.Error(),"E-PLN-004"){t.Fatalf("unknown accepted: %v",err)}
}
func TestCyclesUnknownAndConflicts(t *testing.T){
 c,d:=fixtures()
 b:=c.Packages["base"];b.DependsOn=[]string{"editor"};c.Packages["base"]=b
 if _,err:=Build(c,Selection{IDs:[]string{"editor"}},d,Options{});err==nil||!strings.Contains(err.Error(),"ciclo"){t.Fatalf("cycle: %v",err)}
 c,d=fixtures()
 p:=c.Packages["editor"];p.ConflictsWith=[]string{"git"};c.Packages["editor"]=p
 if _,err:=Build(c,Selection{IDs:[]string{"git","editor"}},d,Options{});err==nil||!strings.Contains(err.Error(),"E-PLN-002"){t.Fatalf("selected conflict: %v",err)}
 d["git"]=domain.Detection{State:domain.StateInstalled}
 if _,err:=Build(c,Selection{IDs:[]string{"editor"}},d,Options{});err==nil||!strings.Contains(err.Error(),"E-PLN-002"){t.Fatalf("installed conflict: %v",err)}
 c,d=fixtures()
 p=c.Packages["git"];p.ConflictsWith=[]string{"editor"};c.Packages["git"]=p
 d["git"]=domain.Detection{State:domain.StateInstalled}
 if _,err:=Build(c,Selection{IDs:[]string{"editor"}},d,Options{});err==nil||!strings.Contains(err.Error(),"E-PLN-002"){t.Fatalf("reverse conflict: %v",err)}
}
func TestProfileCycleAndMissingID(t *testing.T){
 c,d:=fixtures()
 p:=c.Profiles["essencial"];p.Extends="dev";c.Profiles["essencial"]=p
 if _,err:=Build(c,Selection{Profile:"dev"},d,Options{});err==nil{t.Fatal("profile cycle accepted")}
 if _,err:=Build(c,Selection{IDs:[]string{"missing"}},d,Options{});err==nil{t.Fatal("missing ID accepted")}
}
func TestRebootBarrierSplitsOnlyIfActionable(t *testing.T){
 c,d:=fixtures()
 b:=c.Packages["base"];b.Install.RebootBarrier=true;c.Packages["base"]=b
 p,err:=Build(c,Selection{IDs:[]string{"editor"}},d,Options{UpgradeOutdated:true})
 if err!=nil{t.Fatal(err)}
 if len(p.Phases)!=2||!p.Phases[0].RebootAfter||p.Phases[0].Steps[0].PackageID!="base"{t.Fatalf("bad barrier %+v",p)}
 d["base"]=domain.Detection{State:domain.StateInstalled}
 p,err=Build(c,Selection{IDs:[]string{"editor"}},d,Options{UpgradeOutdated:true})
 if err!=nil||len(p.Phases)!=1{t.Fatalf("skipped barrier: %+v err=%v",p,err)}
}
func TestRandomAcyclicTopologicalProperty(t *testing.T){
 rng:=rand.New(rand.NewSource(3211))
 for trial:=0;trial<120;trial++{
 c:=domain.Catalog{Packages:map[string]domain.Package{},Profiles:map[string]domain.Profile{}}
 states:=map[string]domain.Detection{}
 ids:=[]string{"a","b","c","d","e","f","g","h"}
 for i,id:=range ids{
  p:=domain.Package{ID:id}
  for j:=0;j<i;j++{if rng.Intn(3)==0{p.DependsOn=append(p.DependsOn,ids[j])}}
  c.Packages[id]=p
  states[id]=domain.Detection{State:domain.StateAbsent}
 }
 plan,err:=Build(c,Selection{IDs:ids},states,Options{UpgradeOutdated:true})
 if err!=nil{t.Fatal(err)}
 index:=map[string]int{}
 for pos,s:=range flatten(plan){index[s.PackageID]=pos}
 for id,p:=range c.Packages{for _,dep:=range p.DependsOn{if index[dep]>=index[id]{t.Fatalf("%s after %s",dep,id)}}}
 }
}
func TestGoldenJSON(t *testing.T){
 c,d:=fixtures()
 p,err:=Build(c,Selection{IDs:[]string{"editor"}},d,Options{UpgradeOutdated:true})
 if err!=nil{t.Fatal(err)}
 b,err:=json.Marshal(p);if err!=nil{t.Fatal(err)}
 want:=`{"phases":[{"steps":[{"packageId":"base","action":"instalar","reason":"dependência requerida pelo perfil ou pacote"},{"packageId":"browser","action":"instalar","reason":"dependência requerida pelo perfil ou pacote"},{"packageId":"editor","action":"instalar","reason":"selecionado pelo operador"}],"rebootAfter":false}]}`
 if string(b)!=want{t.Fatalf("golden changed %s",b)}
}
