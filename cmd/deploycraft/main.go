// DeployCraft CLI is intentionally read-only during the foundation phase.
// No fake installers, mock success, or system mutations are permitted.
package main

import (
 "flag"
 "fmt"
 "os"
 "sort"

 "github.com/DragonBlack18/DeployCraft/src/catalog"
 "github.com/DragonBlack18/DeployCraft/src/domain"
)

var version="0.1.0-dev"

func main(){
 if len(os.Args)<2{
  fmt.Fprintln(os.Stderr,"DeployCraft",version,"— Fase 0 (somente catálogo).")
  fmt.Fprintln(os.Stderr,"Uso: DeployCraft version | list [--kit <pasta>]")
  os.Exit(domain.ExitUsage)
 }
 switch os.Args[1]{
 case "version": fmt.Println("DeployCraft",version)
 case "list":
  f:=flag.NewFlagSet("list",flag.ContinueOnError)
  kit:=f.String("kit",".","raiz do kit")
  if err:=f.Parse(os.Args[2:]);err!=nil{os.Exit(domain.ExitUsage)}
  c,err:=catalog.Load(*kit)
  if err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(domain.ExitCatalog)}
  ids:=make([]string,0,len(c.Packages));for id:=range c.Packages{ids=append(ids,id)};sort.Strings(ids)
  fmt.Printf("DeployCraft | catálogo %s | %d pacotes (não verificados neste comando)\n",c.Index.CatalogVersion,len(ids))
  for _,id:=range ids{p:=c.Packages[id];fmt.Printf("  %s | %s | %s | %s\n",p.ID,p.Name,p.Version,p.Install.Driver)}
  prof:=make([]string,0,len(c.Profiles));for id:=range c.Profiles{prof=append(prof,id)};sort.Strings(prof)
  fmt.Printf("Perfis (%d):\n",len(prof))
  for _,id:=range prof{fmt.Println("  ",id)}
  fmt.Println("Fase 0: instalar, desinstalar, detectar e retomar ainda não implementados.")
 default:
  fmt.Fprintln(os.Stderr,"Comando não disponível nesta fase. Nenhuma alteração foi feita.")
  os.Exit(domain.ExitUsage)
 }
}
