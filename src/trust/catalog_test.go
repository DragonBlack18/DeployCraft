package trust

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "catalog", "packages"), 0700); err != nil { t.Fatal(err) }
	index := domain.Index{
		Schema:1,CatalogVersion:"2026.10.1",
		Packages:[]string{"packages/example.json"},Profiles:[]domain.Profile{},
	}
	pkg := domain.Package{
		Schema:1,ID:"example",Name:"Offline Example",Version:"1.0",
		Platform:domain.Platform{OS:"windows",Arch:[]string{"x64"}},
		Payload:domain.Payload{Kind:"file",Path:"payloads/example/setup.exe",Size:8,SHA256:strings.Repeat("a",64)},
		Install:domain.Install{Driver:"exe",TimeoutSec:120,SuccessCodes:[]int{0}},
		Detect:domain.Detect{Match:"all",Rules:[]domain.DetectRule{{Type:"file",Path:"C:/Program Files/example.exe"}}},
	}
	for name,data := range map[string]any{"catalog.json":index,filepath.Join("packages","example.json"):pkg} {
		raw,err := json.Marshal(data)
		if err != nil { t.Fatal(err) }
		if err:=os.WriteFile(filepath.Join(root,"catalog",name),raw,0600);err!=nil{t.Fatal(err)}
	}
	return root
}
func TestSignedCatalogVerificationAndTampering(t *testing.T) {
	root:=fixture(t)
	privatePEM,publicPEM,err:=GeneratePEM()
	if err!=nil{t.Fatal(err)}
	priv,err:=ParsePrivatePEM(privatePEM)
	if err!=nil{t.Fatal(err)}
	pub,err:=ParsePublicPEM(publicPEM)
	if err!=nil{t.Fatal(err)}
	sig,err:=Sign(root,priv)
	if err!=nil{t.Fatal(err)}
	path:=filepath.Join(root,"catalog","catalog.sig")
	if err:=os.WriteFile(path,sig,0600);err!=nil{t.Fatal(err)}
	if err:=Verify(root,pub);err!=nil{t.Fatal(err)}
	f,err:=os.OpenFile(filepath.Join(root,"catalog","packages","example.json"),os.O_APPEND|os.O_WRONLY,0600)
	if err!=nil{t.Fatal(err)}
	if _,err:=f.WriteString(" ");err!=nil{t.Fatal(err)}
	if err:=f.Close();err!=nil{t.Fatal(err)}
	if err:=Verify(root,pub);err==nil{t.Fatal("modified manifest accepted")}
}
func TestWrongTrustedKeyAndMissingSignature(t *testing.T) {
	root:=fixture(t)
	key,_ ,err:=GeneratePEM();if err!=nil{t.Fatal(err)}
	priv,err:=ParsePrivatePEM(key);if err!=nil{t.Fatal(err)}
	sig,err:=Sign(root,priv);if err!=nil{t.Fatal(err)}
	_,differentPEM,err:=GeneratePEM();if err!=nil{t.Fatal(err)}
	different,err:=ParsePublicPEM(differentPEM);if err!=nil{t.Fatal(err)}
	if err:=Verify(root,different);err==nil{t.Fatal("unsigned kit accepted")}
	if err:=os.WriteFile(filepath.Join(root,"catalog","catalog.sig"),sig,0600);err!=nil{t.Fatal(err)}
	if err:=Verify(root,different);err==nil{t.Fatal("wrong public key accepted")}
	if err:=Verify(root,ed25519.PublicKey(make([]byte,32)));err==nil{t.Fatal("incorrect pinned key accepted")}
}
func TestUnknownSignatureFieldsRejected(t *testing.T){
	root:=fixture(t)
	privatePEM,_,err:=GeneratePEM();if err!=nil{t.Fatal(err)}
	key,err:=ParsePrivatePEM(privatePEM);if err!=nil{t.Fatal(err)}
	raw,err:=Sign(root,key);if err!=nil{t.Fatal(err)}
	tampered:=strings.Replace(string(raw), "\"schema\": 1", "\"schema\": 1, \"override\": true",1)
	if err:=os.WriteFile(filepath.Join(root,"catalog","catalog.sig"),[]byte(tampered),0600);err!=nil{t.Fatal(err)}
	if err:=Verify(root,key.Public().(ed25519.PublicKey));err==nil{t.Fatal("extra signature properties accepted")}
}
