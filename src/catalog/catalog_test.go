package catalog

import (
 "encoding/json"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/DragonBlack18/DeployCraft/src/domain"
)
func examplePackage(id string)domain.Package{
 return domain.Package{
  Schema:1, ID:id, Name:"Teste "+id,Version:"1.0",Publisher:"Teste",
  Platform:domain.Platform{OS:"windows",Arch:[]string{"x64"},MinBuild:17763},
  Payload:domain.Payload{Kind:"file",Path:"payloads/"+id+"/"+id+".exe",Size:2,SHA256:strings.Repeat("a",64)},
  Install:domain.Install{Driver:"exe",Args:[]json.RawMessage{json.RawMessage(`"/S"`)},TimeoutSec:60,SuccessCodes:[]int{0}},
  Detect:domain.Detect{Match:"all",Rules:[]domain.DetectRule{{Type:"file",Path:"%ProgramFiles%/example.exe"}}},
 }
}
func writeCatalog(t *testing.T,packages []domain.Package,profiles []domain.Profile)string{
 t.Helper()
 root:=t.TempDir()
 if err:=os.MkdirAll(filepath.Join(root,"catalog","packages"),0700);err!=nil{t.Fatal(err)}
 idx:=domain.Index{Schema:1,CatalogVersion:"2026.10.1",Packages:[]string{},Profiles:profiles}
 for _,p:=range packages{
  idx.Packages=append(idx.Packages,"packages/"+p.ID+".json")
  data,_:=json.Marshal(p)
  if err:=os.WriteFile(filepath.Join(root,"catalog","packages",p.ID+".json"),data,0600);err!=nil{t.Fatal(err)}
 }
 data,_:=json.Marshal(idx)
 if err:=os.WriteFile(filepath.Join(root,"catalog","catalog.json"),data,0600);err!=nil{t.Fatal(err)}
 return root
}
func TestLoadValid(t *testing.T){
 root:=writeCatalog(t,[]domain.Package{examplePackage("alpha")},[]domain.Profile{{ID:"essencial",Name:"Essencial",Packages:[]string{"alpha"}}})
 c,err:=Load(root)
 if err!=nil{t.Fatal(err)}
 if len(c.Packages)!=1 || len(c.Profiles)!=1{t.Fatalf("catálogo incorreto: %+v",c)}
}
func TestLoadRejectsUnknownPackageField(t *testing.T){
 root:=writeCatalog(t,[]domain.Package{examplePackage("alpha")},nil)
 file:=filepath.Join(root,"catalog","packages","alpha.json")
 data,err:=os.ReadFile(file);if err!=nil{t.Fatal(err)}
 data=[]byte(strings.Replace(string(data),`"schema":1`,`"schema":1,"unexpected":true`,1))
 if err:=os.WriteFile(file,data,0600);err!=nil{t.Fatal(err)}
 if _,err:=Load(root);err==nil{t.Fatal("campo desconhecido deveria falhar")}
}
func TestLoadRejectsDependencyCycle(t *testing.T){
 a,b:=examplePackage("alpha"),examplePackage("beta")
 a.DependsOn=[]string{"beta"}
 b.DependsOn=[]string{"alpha"}
 if _,err:=Load(writeCatalog(t,[]domain.Package{a,b},nil));err==nil || !strings.Contains(err.Error(),"E-CAT-007"){t.Fatalf("esperado E-CAT-007, obtido: %v",err)}
}
func TestLoadRejectsUnknownReference(t *testing.T){
 p:=examplePackage("alpha");p.DependsOn=[]string{"nao-existe"}
 if _,err:=Load(writeCatalog(t,[]domain.Package{p},nil));err==nil || !strings.Contains(err.Error(),"E-CAT-006"){t.Fatalf("esperado E-CAT-006, obtido: %v",err)}
}
func TestSafeRelative(t *testing.T){
 for _,s:=range []string{"../a","C:/Windows/a","/tmp/x","x/../../y",`x\y`,"a//b","."}{
  if err:=safeRelative(s);err==nil{t.Errorf("aceitou caminho perigoso %q",s)}
 }
 for _,s:=range []string{"payloads/alpha/app.exe","packages/alpha.json"}{if err:=safeRelative(s);err!=nil{t.Fatal(err)}}
}
