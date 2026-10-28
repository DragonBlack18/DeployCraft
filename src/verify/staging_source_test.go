package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)
type memorySource map[string][]byte
func (s memorySource)Open(name string)(io.ReadCloser,error){
	data,ok:=s[name]
	if !ok{return nil,fmt.Errorf("not found")}
	return io.NopCloser(strings.NewReader(string(data))),nil
}
func TestStageSourceSelectivelyCopiesAndHashes(t *testing.T){
	content:=[]byte("fixture-not-executable")
	sum:=sha256.Sum256(content)
	src:=memorySource{"payloads/demo/setup.exe":content,"payloads/another.exe":[]byte("not selected")}
	p:=domain.Package{ID:"demo",
		Payload:domain.Payload{Kind:"file",Path:"payloads/demo/setup.exe",Size:int64(len(content)),SHA256:hex.EncodeToString(sum[:])},
		Install:domain.Install{Driver:"exe"},
	}
	root:=filepath.Join(t.TempDir(),"stage")
	file,err:=StageSource(src,root,p)
	if err!=nil{t.Fatal(err)}
	got,err:=os.ReadFile(file);if err!=nil{t.Fatal(err)}
	if string(got)!=string(content){t.Fatalf("wrong copied contents %q",got)}
	if _,err:=os.Stat(filepath.Join(root,"another.exe"));!os.IsNotExist(err){t.Fatal("unselected entry extracted")}
	src["payloads/demo/setup.exe"]=[]byte("modified")
	_,err=StageSource(src,filepath.Join(t.TempDir(),"other"),p)
	if err==nil{t.Fatal("tampered source accepted")}
}
