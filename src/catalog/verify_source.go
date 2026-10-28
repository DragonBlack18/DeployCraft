package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

// VerifySource checks payload bytes provided by a FolderSource or PackSource.
// It does not extract files or start executable processes.
func VerifySource(src domain.PayloadSource,c domain.Catalog) []VerifyIssue{
	issues:=[]VerifyIssue{}
	ids:=make([]string,0,len(c.Packages))
	for id:=range c.Packages{ids=append(ids,id)}
	sort.Strings(ids)
	for _,id:=range ids{
		p:=c.Packages[id]
		if p.Payload.Kind=="dir"{
			for _,f:=range p.Payload.Files{
				rel:=p.Payload.Path+"/"+f.Path
				if issue:=verifySourceEntry(src,id,rel,f.Size,f.SHA256);issue!=nil{issues=append(issues,*issue)}
			}
		}else{
			if issue:=verifySourceEntry(src,id,p.Payload.Path,p.Payload.Size,p.Payload.SHA256);issue!=nil{issues=append(issues,*issue)}
		}
	}
	return issues
}
func verifySourceEntry(src domain.PayloadSource,id,rel string,expectedSize int64,expectedHash string)*VerifyIssue{
	fail:=func(code,msg string)*VerifyIssue{return &VerifyIssue{PackageID:id,Path:rel,Code:code,Problem:msg}}
	if err:=safeRelative(rel);err!=nil{return fail("E-VER-003",err.Error())}
	if src==nil{return fail("E-VER-003","PayloadSource ausente")}
	f,err:=src.Open(rel)
	if err!=nil{return fail("E-VER-003",fmt.Sprintf("fonte indisponível: %v",err))}
	defer f.Close()
	hasher:=sha256.New()
	size,err:=io.CopyBuffer(hasher,f,make([]byte,1<<20))
	if err!=nil{return fail("E-VER-003",err.Error())}
	if size!=expectedSize{return fail("E-VER-002",fmt.Sprintf("tamanho esperado %d, obtido %d",expectedSize,size))}
	if !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)),expectedHash){
		return fail("E-VER-001","SHA-256 do payload difere do catálogo")
	}
	return nil
}
