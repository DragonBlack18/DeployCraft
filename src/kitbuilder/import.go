// Package kitbuilder imports approved offline payloads and creates distributable
// kits. No network access, no shell and no installation is performed by this package.
package kitbuilder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/catalog"
	"github.com/DragonBlack18/DeployCraft/src/domain"
)

type ImportOptions struct {
	Root, Input, ID, Name, Version, Driver string
	Args []string
	DetectName, DetectFile string
	SourceURL, VendorHash string
	Profiles []string
}
type UpdateOptions struct {
	Root, Input, ID, Version string
	SourceURL, VendorHash string
}
var validIdentifier = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var validFilename = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func absRoot(root string)(string,error){
	if root==""{root="."}
	absolute,err:=filepath.Abs(root)
	if err!=nil{return "",err}
	return filepath.EvalSymlinks(absolute)
}
func regularFile(file string)(os.FileInfo,error){
	info,err:=os.Lstat(file)
	if err!=nil{return nil,err}
	if !info.Mode().IsRegular(){return nil,fmt.Errorf("E-BLD-001: arquivo não é regular: %s",file)}
	return info,nil
}
func checkInput(file string,driver string)(string,error){
	if driver!="msi" && driver!="exe"{return "",fmt.Errorf("E-BLD-001: use driver msi ou exe para importar")}
	abs,err:=filepath.Abs(file)
	if err!=nil{return "",err}
	if _,err:=regularFile(abs);err!=nil{return "",err}
	base:=filepath.Base(abs)
	if !validFilename.MatchString(base) {
		return "",fmt.Errorf("E-BLD-001: renomeie a origem sem espaços ou acentos: %s",base)
	}
	if strings.ToLower(filepath.Ext(base))!="."+driver {
		return "",fmt.Errorf("E-BLD-001: extensão da origem incompatível com driver %s",driver)
	}
	return abs,nil
}
func stagedCopy(source,dest string)(hash string,size int64,err error){
	_,statErr:=os.Lstat(dest)
	if statErr==nil{return "",0,fmt.Errorf("E-BLD-002: destino já existe: %s",dest)}
	if !os.IsNotExist(statErr){return "",0,statErr}
	if err:=os.MkdirAll(filepath.Dir(dest),0700);err!=nil{return "",0,err}
	in,err:=os.Open(source);if err!=nil{return "",0,err}
	defer in.Close()
	tmp,err:=os.CreateTemp(filepath.Dir(dest),".deploycraft-copy-*")
	if err!=nil{return "",0,err}
	tempName:=tmp.Name()
	defer os.Remove(tempName)
	if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return "",0,err}
	h:=sha256.New()
	size,err=io.CopyBuffer(io.MultiWriter(tmp,h),in,make([]byte,1<<20))
	if err!=nil{tmp.Close();return "",0,err}
	if err:=tmp.Sync();err!=nil{tmp.Close();return "",0,err}
	if err:=tmp.Close();err!=nil{return "",0,err}
	if size==0{return "",0,fmt.Errorf("E-BLD-001: origem vazia")}
	if _,err:=os.Lstat(dest);err==nil{return "",0,fmt.Errorf("E-BLD-002: destino já existe")}
	if err:=os.Rename(tempName,dest);err!=nil{return "",0,err}
	return hex.EncodeToString(h.Sum(nil)),size,nil
}
func atomicJSON(file string,value any)error{
	data,err:=json.MarshalIndent(value,"","  ")
	if err!=nil{return err}
	data=append(data,'\n')
	if err:=os.MkdirAll(filepath.Dir(file),0700);err!=nil{return err}
	tmp,err:=os.CreateTemp(filepath.Dir(file),".deploycraft-json-*")
	if err!=nil{return err}
	name:=tmp.Name();defer os.Remove(name)
	if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
	if _,err:=tmp.Write(data);err!=nil{tmp.Close();return err}
	if err:=tmp.Sync();err!=nil{tmp.Close();return err}
	if err:=tmp.Close();err!=nil{return err}
	return os.Rename(name,file)
}
func catalogIndexPath(root string)string{return filepath.Join(root,"catalog","catalog.json")}
func packagePath(root,id string)string{return filepath.Join(root,"catalog","packages",id+".json")}
func payloadDestination(root,id,version,source string)(string,string){
	name:=version+"-"+filepath.Base(source)
	relative:=path.Join("payloads",id,name)
	return relative,filepath.Join(root,filepath.FromSlash(relative))
}
func validateVendorHash(vendor,actual string)error{
	if vendor==""{return nil}
	if len(vendor)!=64 || !strings.EqualFold(vendor,actual){
		return fmt.Errorf("E-BLD-004: SHA-256 da origem diferente do hash informado do fabricante")
	}
	return nil
}

// Add requires explicit, reviewable installation switches and detection rules.
// It does not guess MSI ProductCode or claim a real installer was tested.
func Add(o ImportOptions)(domain.Package,error){
	var empty domain.Package
	root,err:=absRoot(o.Root);if err!=nil{return empty,err}
	c,err:=catalog.Load(root);if err!=nil{return empty,err}
	if !validIdentifier.MatchString(o.ID) || o.Name=="" || !domain.ValidVersion(o.Version){
		return empty,fmt.Errorf("E-BLD-001: id/nome/versão inválidos")
	}
	if _,exists:=c.Packages[o.ID];exists{return empty,fmt.Errorf("E-BLD-002: %s já cadastrado; use update",o.ID)}
	if (o.DetectName=="")==(o.DetectFile==""){
		return empty,fmt.Errorf("E-BLD-001: informe exatamente uma detecção --detect-name ou --detect-file")
	}
	if o.Driver=="exe" && len(o.Args)==0{
		return empty,fmt.Errorf("E-BLD-001: exe exige ao menos um switch informado explicitamente")
	}
	for _,profile:=range o.Profiles{
		if _,ok:=c.Profiles[profile];!ok{return empty,fmt.Errorf("E-BLD-001: perfil inexistente: %s",profile)}
	}
	source,err:=checkInput(o.Input,o.Driver);if err!=nil{return empty,err}
	relative,dest:=payloadDestination(root,o.ID,o.Version,source)
	hash,size,err:=stagedCopy(source,dest)
	if err!=nil{return empty,err}
	defer func(){if err!=nil{os.Remove(dest)}}()
	if err=validateVendorHash(o.VendorHash,hash);err!=nil{return empty,err}
	args:=make([]json.RawMessage,0,len(o.Args))
	for _,v:=range o.Args{
		if v=="" || strings.Contains(v,"{secret}"){err=fmt.Errorf("E-BLD-001: argumento vazio/secret não suportado no add");return empty,err}
		data,_:=json.Marshal(v)
		args=append(args,data)
	}
	rule:=domain.DetectRule{}
	if o.DetectName!=""{
		rule=domain.DetectRule{Type:"registry-uninstall",DisplayName:o.DetectName,MinVersion:o.Version}
	}else{
		rule=domain.DetectRule{Type:"file",Path:o.DetectFile}
	}
	p:=domain.Package{
		Schema:1,ID:o.ID,Name:o.Name,Version:o.Version,
		Platform:domain.Platform{OS:"windows",Arch:[]string{"x64"},MinBuild:17763},
		Payload:domain.Payload{Kind:"file",Path:relative,SHA256:hash,Size:size},
		Install:domain.Install{Driver:o.Driver,Args:args,TimeoutSec:600,SuccessCodes:[]int{0},
			RebootCodes:[]int{3010,1641},RetryCodes:[]int{1618}},
		Detect:domain.Detect{Match:"all",Rules:[]domain.DetectRule{rule}},
		Provenance:&domain.Provenance{Source:o.SourceURL,VendorHash:o.VendorHash,AddedOn:time.Now().UTC().Format("2006-01-02")},
	}
	c.Index.Packages=append(c.Index.Packages,path.Join("packages",o.ID+".json"))
	for i:=range c.Index.Profiles{
		for _,profile:=range o.Profiles{
			if c.Index.Profiles[i].ID==profile{
				found:=false
				for _,id:=range c.Index.Profiles[i].Packages{if id==o.ID{found=true}}
				if !found{c.Index.Profiles[i].Packages=append(c.Index.Profiles[i].Packages,o.ID)}
			}
		}
	}
	manifest:=packagePath(root,o.ID)
	if _,e:=os.Lstat(manifest);e==nil{
		err=fmt.Errorf("E-BLD-002: manifesto já existe");return empty,err
	}
	if err=atomicJSON(manifest,p);err!=nil{return empty,err}
	oldIndex:=domain.Index{}
	indexWritten:=false
	defer func(){
		if err!=nil{
			if indexWritten{_ =atomicJSON(catalogIndexPath(root),oldIndex)}
			_ =os.Remove(manifest)
		}
	}()
	// Keep the previous index so a rejected edit cannot leave broken references.
	oldIndex.Schema=c.Index.Schema
	oldIndex.CatalogVersion=c.Index.CatalogVersion
	oldIndex.Packages=append([]string(nil),c.Index.Packages[:len(c.Index.Packages)-1]...)
	for _,profile:=range c.Index.Profiles{
		copyProfile:=profile
		copyProfile.Packages=make([]string,0,len(profile.Packages))
		for _,id:=range profile.Packages{if id!=o.ID{copyProfile.Packages=append(copyProfile.Packages,id)}}
		oldIndex.Profiles=append(oldIndex.Profiles,copyProfile)
	}
	if err=atomicJSON(catalogIndexPath(root),c.Index);err!=nil{return empty,err}
	indexWritten=true
	if _,err=catalog.Load(root);err!=nil{
		_ =atomicJSON(catalogIndexPath(root),oldIndex)
		return empty,fmt.Errorf("E-BLD-003: novo catálogo recusado: %w",err)
	}
	return p,nil
}

// Update keeps prior payloads as immutable local files; the distributable kit
// includes only the new version referenced by the updated manifest.
func Update(o UpdateOptions)(domain.Package,error){
	var empty domain.Package
	root,err:=absRoot(o.Root);if err!=nil{return empty,err}
	c,err:=catalog.Load(root);if err!=nil{return empty,err}
	p,ok:=c.Packages[o.ID];if !ok{return empty,fmt.Errorf("E-PLN-001: pacote inexistente: %s",o.ID)}
	if !domain.ValidVersion(o.Version){return empty,fmt.Errorf("E-BLD-001: versão inválida")}
	cmp,err:=domain.CompareVersions(o.Version,p.Version)
	if err!=nil{return empty,err}
	if cmp<=0{return empty,fmt.Errorf("E-BLD-001: update exige versão maior que %s",p.Version)}
	if p.Payload.Kind!="file" || (p.Install.Driver!="exe" && p.Install.Driver!="msi"){
		return empty,fmt.Errorf("E-BLD-001: update inicial disponível apenas para payload file MSI/EXE")
	}
	source,err:=checkInput(o.Input,p.Install.Driver);if err!=nil{return empty,err}
	relative,dest:=payloadDestination(root,o.ID,o.Version,source)
	hash,size,err:=stagedCopy(source,dest);if err!=nil{return empty,err}
	defer func(){if err!=nil{_ =os.Remove(dest)}}()
	if err=validateVendorHash(o.VendorHash,hash);err!=nil{return empty,err}
	oldVersion:=p.Version
	p.Version=o.Version
	p.Payload=domain.Payload{Kind:"file",Path:relative,SHA256:hash,Size:size}
	for i:=range p.Detect.Rules {
		if p.Detect.Rules[i].MinVersion==oldVersion{p.Detect.Rules[i].MinVersion=o.Version}
	}
	if p.Provenance==nil{p.Provenance=&domain.Provenance{}}
	if o.SourceURL!=""{p.Provenance.Source=o.SourceURL}
	p.Provenance.VendorHash=o.VendorHash
	p.Provenance.AddedOn=time.Now().UTC().Format("2006-01-02")
	manifest:=packagePath(root,o.ID)
	previous,err:=os.ReadFile(manifest);if err!=nil{return empty,err}
	if err=atomicJSON(manifest,p);err!=nil{return empty,err}
	if _,err=catalog.Load(root);err!=nil{
		_ =os.WriteFile(manifest,previous,0600)
		return empty,fmt.Errorf("E-BLD-003: update recusado: %w",err)
	}
	return p,nil
}
