package kitbuilder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/pack"
)

func compileSFXEngine(root,out,publicHex string)error{
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Minute)
	defer cancel()
	ldflags:="-s -w -X main.sfxExpected=1"
	if publicHex!=""{ldflags+=" -X main.trustedCatalogPublicKeyHex="+publicHex}
	cmd:=exec.CommandContext(ctx,"go","build","-trimpath","-ldflags",ldflags,"-o",out,"./cmd/deploycraft")
	cmd.Dir=root
	cmd.Env=append(os.Environ(),"GOOS=windows","GOARCH=amd64","CGO_ENABLED=0")
	output,err:=cmd.CombinedOutput()
	if err!=nil{return fmt.Errorf("E-BLD-006: compilação do SFX falhou: %w: %s",err,string(output))}
	return nil
}
func filesForPack(root string)([]string,error){
	files:=[]string{}
	for _,sub:=range []string{"catalog","payloads"}{
		parent:=filepath.Join(root,sub)
		if _,err:=os.Stat(parent);os.IsNotExist(err){continue}else if err!=nil{return nil,err}
		err:=filepath.WalkDir(parent,func(name string,entry os.DirEntry,err error)error{
			if err!=nil{return err}
			if entry.IsDir(){return nil}
			info,err:=entry.Info();if err!=nil{return err}
			if !info.Mode().IsRegular(){return fmt.Errorf("E-SFX-001: entrada não regular: %s",name)}
			rel,err:=filepath.Rel(root,name);if err!=nil{return err}
			rel=filepath.ToSlash(rel)
			if strings.HasPrefix(rel,"catalog/")||strings.HasPrefix(rel,"payloads/"){
				files=append(files,rel)
			}
			return nil
		})
		if err!=nil{return nil,err}
	}
	sort.Strings(files)
	return files,nil
}
// BuildSFX creates a single Windows executable with an uncompressed indexed
// overlay. All files remain independently stream-readable via PackSource.
// Authenticode is NOT applied: this is a LAB artifact until code-signing and
// post-reboot VM tests have been completed.
func BuildSFX(o BuildOptions)(BuildReport,error){
	var report BuildReport
	root,err:=absRoot(o.Root);if err!=nil{return report,err}
	if o.Destination==""{return report,fmt.Errorf("E-BLD-002: --out obrigatório")}
	dest,err:=filepath.Abs(o.Destination);if err!=nil{return report,err}
	if filepath.Ext(dest)!=".exe"{return report,fmt.Errorf("E-BLD-002: SFX exige extensão .exe")}
	if _,err:=os.Lstat(dest);err==nil{return report,fmt.Errorf("E-BLD-002: destino já existe")}else if !os.IsNotExist(err){return report,err}
	if inside(root,dest)&&!inside(filepath.Join(root,"dist"),dest){
		return report,fmt.Errorf("E-BLD-002: dentro do repositório use apenas dist/")
	}
	parent:=filepath.Dir(dest)
	if err:=os.MkdirAll(parent,0700);err!=nil{return report,err}
	staging,err:=os.MkdirTemp(parent,".deploycraft-sfx-*")
	if err!=nil{return report,err}
	defer os.RemoveAll(staging)
	folder:=filepath.Join(staging,"kit")
	folderReport,err:=buildFolder(BuildOptions{
		Root:root,Destination:folder,PrivateKeyPath:o.PrivateKeyPath,LabUnsigned:o.LabUnsigned,
	},compileWindowsEngine)
	if err!=nil{return report,err}
	rels,err:=filesForPack(folder);if err!=nil{return report,err}
	// The generated folder motor is not embedded; the SFX motor is recompiled
	// with sfxExpected=1 and the identical catalog trust anchor.
	outPath:=filepath.Join(staging,"DeployCraft-SFX.exe")
	if err:=compileSFXEngine(root,outPath,folderReport.PublicKeyHex);err!=nil{return report,err}
	output,err:=os.OpenFile(outPath,os.O_RDWR,0);if err!=nil{return report,err}
	_,err=pack.Append(output,folder,rels)
	closeErr:=output.Close()
	if err!=nil{return report,err}
	if closeErr!=nil{return report,closeErr}
	reader,err:=pack.OpenFile(outPath);if err!=nil{return report,err}
	err=reader.VerifyAll()
	reader.Close()
	if err!=nil{return report,err}
	f,err:=os.Open(outPath);if err!=nil{return report,err}
	h:=sha256.New()
	_,copyErr:=io.CopyBuffer(h,f,make([]byte,1<<20))
	f.Close()
	if copyErr!=nil{return report,copyErr}
	if _,err:=os.Lstat(dest);err==nil{return report,fmt.Errorf("E-BLD-002: destino criado por outra execução")}
	if err:=os.Rename(outPath,dest);err!=nil{return report,err}
	report=folderReport
	report.Destination=dest
	report.EngineSHA256=hex.EncodeToString(h.Sum(nil))
	return report,nil
}
