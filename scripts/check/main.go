// Command check is the single reproducible quality gate for local and CI use.
package main

import (
 "errors"
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
)

const (
 motorLimit=15<<20
 catalogLimit=1<<20
 trackedFileLimit=1<<20
)
func command(env []string,name string,args ...string)error{
 c:=exec.Command(name,args...)
 c.Env=append(os.Environ(),env...)
 out,err:=c.CombinedOutput()
 if err!=nil{return fmt.Errorf("%s %s: %w\n%s",name,strings.Join(args," "),err,out)}
 return nil
}
func checkFormat()error{
 out,err:=exec.Command("gofmt","-l",".").CombinedOutput()
 if err!=nil{return err}
 if s:=strings.TrimSpace(string(out));s!=""{return fmt.Errorf("gofmt: execute gofmt -w nos arquivos:\n%s",s)}
 return nil
}
func checkGit()error{
 if _,err:=os.Stat(".git");err!=nil{return nil}
 out,err:=exec.Command("git","ls-files","payloads","dist").CombinedOutput()
 if err!=nil{return err}
 if strings.TrimSpace(string(out))!=""{return fmt.Errorf("payloads/ e dist/ estão rastreados: %s",out)}
 out,err=exec.Command("git","ls-files","-z").Output();if err!=nil{return err}
 for _,s:=range strings.Split(string(out),"\x00"){
  if s==""{continue}
  info,err:=os.Stat(s)
  if err==nil && info.Size()>trackedFileLimit{return fmt.Errorf("arquivo grande rastreado: %s",s)}
 }
 return nil
}
func checkBudget()error{
 temp,err:=os.MkdirTemp("","deploycraft-check-*")
 if err!=nil{return err};defer os.RemoveAll(temp)
 binary:=filepath.Join(temp,"DeployCraft.exe")
 if err:=command([]string{"GOOS=windows","GOARCH=amd64"},"go","build","-trimpath","-ldflags","-s -w","-o",binary,"./cmd/deploycraft");err!=nil{return err}
 info,err:=os.Stat(binary);if err!=nil{return err}
 if info.Size()>motorLimit{return fmt.Errorf("motor %d bytes excede orçamento %d",info.Size(),motorLimit)}
 var total int64
 err=filepath.WalkDir("catalog",func(p string,e os.DirEntry,err error)error{
  if err!=nil{return err}
  if e.IsDir(){return nil}
  info,err:=e.Info();if err!=nil{return err}
  total+=info.Size();return nil
 })
 if err!=nil{return err}
 if total>catalogLimit{return fmt.Errorf("catálogo %d bytes excede orçamento %d",total,catalogLimit)}
 fmt.Printf("tamanho do motor: %d bytes; catálogo: %d bytes\n",info.Size(),total)
 return nil
}
func checkPayloads()error{
 if _,err:=os.Stat("payloads");errors.Is(err,os.ErrNotExist){fmt.Println("sem payloads físicos neste checkout; verify não se aplica");return nil}
 return command(nil,"go","run","./cmd/kit-builder","verify","--root",".")
}
func main(){
 checks:=[]struct{name string;run func()error}{
  {"gofmt",checkFormat},
  {"go vet",func()error{return command(nil,"go","vet","./...")}},
  {"build nativo",func()error{return command(nil,"go","build","./...")}},
  {"build Windows amd64",func()error{return command([]string{"GOOS=windows","GOARCH=amd64"},"go","build","./...")}},
  {"testes de unidade",func()error{return command(nil,"go","test","./...")}},
  {"nenhum binário pesado versionado",checkGit},
  {"lint do catálogo",func()error{return command(nil,"go","run","./cmd/kit-builder","lint","--root",".")}},
  {"verificação dos payloads locais",checkPayloads},
  {"orçamento do motor e catálogo",checkBudget},
 }
 for _,c:=range checks{
  fmt.Println("==",c.name)
  if err:=c.run();err!=nil{fmt.Fprintln(os.Stderr,"FAIL:",err);os.Exit(1)}
  fmt.Println("PASS:",c.name)
 }
 fmt.Println("DEPLOYCRAFT QUALITY GATE: PASS")
}
