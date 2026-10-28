package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/pack"
	"github.com/DragonBlack18/DeployCraft/src/trust"
)

// openEmbeddedKit materializes ONLY the small signed catalog. Payload bytes
// remain inside the SFX and are streamed on demand into verified staging.
func openEmbeddedKit()(string,*pack.Reader,error){
	exe,err:=os.Executable()
	if err!=nil{return "",nil,err}
	reader,err:=pack.OpenFile(exe)
	if err!=nil{return "",nil,fmt.Errorf("E-SFX-001: SFX inválido: %w",err)}
	fail:=func(err error)(string,*pack.Reader,error){reader.Close();return "",nil,err}
	cache,err:=os.UserCacheDir()
	if err!=nil{return fail(err)}
	root:=filepath.Join(cache,"DeployCraft","SFX",reader.DigestHex())
	for _,entry:=range reader.Files(){
		if !strings.HasPrefix(entry.Path,"catalog/"){continue}
		rel:=strings.TrimPrefix(entry.Path,"catalog/")
		if rel=="" || strings.HasPrefix(rel,"../") || path.Clean(rel)!=rel{
			return fail(fmt.Errorf("E-SFX-001: catálogo fora da pasta: %s",entry.Path))
		}
		out:=filepath.Join(root,filepath.FromSlash(entry.Path))
		if err:=os.MkdirAll(filepath.Dir(out),0700);err!=nil{return fail(err)}
		if info,err:=os.Lstat(out);err==nil{
			if !info.Mode().IsRegular() || info.Size()!=entry.Size{
				return fail(fmt.Errorf("E-SFX-002: cache de catálogo alterado: %s",entry.Path))
			}
			f,err:=os.Open(out);if err!=nil{return fail(err)}
			h:=sha256.New()
			n,hashErr:=io.CopyBuffer(h,f,make([]byte,128<<10))
			f.Close()
			if hashErr!=nil||n!=entry.Size||hex.EncodeToString(h.Sum(nil))!=entry.SHA256{
				return fail(fmt.Errorf("E-SFX-002: cache de catálogo divergente: %s",entry.Path))
			}
			continue
		}else if !os.IsNotExist(err){return fail(err)}
		source,err:=reader.Open(entry.Path);if err!=nil{return fail(err)}
		dst,err:=os.OpenFile(out,os.O_WRONLY|os.O_CREATE|os.O_EXCL,0600)
		if err!=nil{source.Close();return fail(err)}
		h:=sha256.New()
		n,copyErr:=io.CopyBuffer(io.MultiWriter(dst,h),source,make([]byte,128<<10))
		closeErr:=dst.Close()
		source.Close()
		if copyErr!=nil||closeErr!=nil||n!=entry.Size||hex.EncodeToString(h.Sum(nil))!=entry.SHA256{
			os.Remove(out)
			return fail(fmt.Errorf("E-SFX-002: catálogo do SFX não confere: %s",entry.Path))
		}
	}
	key,err:=pinnedPublicKey()
	if err!=nil{return fail(err)}
	if len(key)>0{
		if err:=trust.Verify(root,key);err!=nil{return fail(err)}
	}
	return root,reader,nil
}
