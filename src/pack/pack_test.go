package pack

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
func createPack(t *testing.T)(string,string){
	t.Helper()
	root:=t.TempDir()
	for rel,data:=range map[string]string{
		"catalog/catalog.json":"catalog",
		"payloads/example/setup.exe":"fake-only, never executed",
	}{
		file:=filepath.Join(root,filepath.FromSlash(rel))
		if err:=os.MkdirAll(filepath.Dir(file),0700);err!=nil{t.Fatal(err)}
		if err:=os.WriteFile(file,[]byte(data),0600);err!=nil{t.Fatal(err)}
	}
	output:=filepath.Join(t.TempDir(),"fixture.exe")
	f,err:=os.Create(output);if err!=nil{t.Fatal(err)}
	if _,err:=f.Write([]byte("test stub"));err!=nil{t.Fatal(err)}
	if _,err:=Append(f,root,[]string{"payloads/example/setup.exe","catalog/catalog.json"});err!=nil{t.Fatal(err)}
	if err:=f.Close();err!=nil{t.Fatal(err)}
	return root,output
}
func TestStreamingRoundtrip(t *testing.T){
	root,file:=createPack(t)
	r,err:=OpenFile(file);if err!=nil{t.Fatal(err)}
	defer r.Close()
	if err:=r.VerifyAll();err!=nil{t.Fatal(err)}
	for _,entry:=range r.Files(){
		stream,err:=r.Open(entry.Path);if err!=nil{t.Fatal(err)}
		data,err:=io.ReadAll(stream);stream.Close()
		if err!=nil{t.Fatal(err)}
		want,err:=os.ReadFile(filepath.Join(root,filepath.FromSlash(entry.Path)));if err!=nil{t.Fatal(err)}
		if string(data)!=string(want){t.Fatalf("bad entry %s",entry.Path)}
	}
	if _,err:=r.Open("../escape");err==nil{t.Fatal("traversal accepted")}
}
func TestTamperPayloadAndIndex(t *testing.T){
	_,file:=createPack(t)
	r,err:=OpenFile(file);if err!=nil{t.Fatal(err)}
	var target Entry
	for _,e:=range r.Files(){if strings.HasPrefix(e.Path,"payloads/"){target=e}}
	r.Close()
	f,err:=os.OpenFile(file,os.O_RDWR,0);if err!=nil{t.Fatal(err)}
	if _,err:=f.WriteAt([]byte("!"),target.Offset);err!=nil{t.Fatal(err)}
	f.Close()
	r,err=OpenFile(file);if err!=nil{t.Fatal(err)}
	if err:=r.Verify(target.Path);err==nil{t.Fatal("modified payload accepted")}
	r.Close()
	b,err:=os.ReadFile(file);if err!=nil{t.Fatal(err)}
	start:=int(binary.LittleEndian.Uint64(b[len(b)-footerSize+8:]))
	b[start]^=1
	if err:=os.WriteFile(file,b,0600);err!=nil{t.Fatal(err)}
	if r,err:=OpenFile(file);err==nil{r.Close();t.Fatal("modified index accepted")}
}
func TestForgedTraversalIndexRejected(t *testing.T){
	_,file:=createPack(t)
	b,err:=os.ReadFile(file);if err!=nil{t.Fatal(err)}
	start:=int(binary.LittleEndian.Uint64(b[len(b)-footerSize+8:]))
	var index Index
	if err:=json.Unmarshal(b[start:len(b)-footerSize],&index);err!=nil{t.Fatal(err)}
	index.Files[0].Path="../outside"
	raw,err:=json.Marshal(index);if err!=nil{t.Fatal(err)}
	footer:=make([]byte,footerSize)
	copy(footer,magic)
	binary.LittleEndian.PutUint64(footer[8:16],uint64(start))
	binary.LittleEndian.PutUint64(footer[16:24],uint64(len(raw)))
	hash:=sha256.Sum256(raw)
	copy(footer[24:],hash[:])
	forged:=append(append(b[:start],raw...),footer...)
	if err:=os.WriteFile(file,forged,0600);err!=nil{t.Fatal(err)}
	if r,err:=OpenFile(file);err==nil{r.Close();t.Fatal("forged unsafe index accepted")}
}
func TestRejectsBadAppendPathsAndSupportsPadding(t *testing.T){
	root,file:=createPack(t)
	out,err:=os.CreateTemp(t.TempDir(),"bad-*");if err!=nil{t.Fatal(err)}
	for _,list:=range [][]string{
		{"../a"},{"C:/a"},{"/a"},{"catalog/catalog.json","catalog/catalog.json"},
	}{
		if _,err:=Append(out,root,list);err==nil{t.Fatalf("unsafe list accepted: %v",list)}
	}
	out.Close()
	f,err:=os.OpenFile(file,os.O_APPEND|os.O_WRONLY,0);if err!=nil{t.Fatal(err)}
	if _,err:=f.Write(make([]byte,7));err!=nil{t.Fatal(err)}
	f.Close()
	r,err:=OpenFile(file);if err!=nil{t.Fatal(err)}
	defer r.Close()
	if err:=r.VerifyAll();err!=nil{t.Fatal(err)}
}
