package kitbuilder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DragonBlack18/DeployCraft/src/pack"
)

func TestBuildSFXCreatesIndexedWindowsExecutable(t *testing.T){
	root:=fixtureRoot(t)
	addExample(t,root)
	keys:=t.TempDir()
	private:=filepath.Join(keys,"signing-private.pem")
	public:=filepath.Join(keys,"signing-public.pem")
	if err:=GenerateSigningKeys(root,private,public);err!=nil{t.Fatal(err)}
	dest:=filepath.Join(t.TempDir(),"DeployCraft-SFX.exe")
	report,err:=BuildSFX(BuildOptions{Root:root,Destination:dest,PrivateKeyPath:private})
	if err!=nil{t.Fatal(err)}
	if !report.Signed||report.PackageCount!=1||report.PublicKeyHex==""{
		t.Fatalf("unexpected build report: %+v",report)
	}
	f,err:=os.Open(dest);if err!=nil{t.Fatal(err)}
	header:=make([]byte,2)
	if _,err:=f.Read(header);err!=nil{t.Fatal(err)}
	f.Close()
	if string(header)!="MZ"{t.Fatal("SFX is not Windows PE")}
	reader,err:=pack.OpenFile(dest)
	if err!=nil{t.Fatal(err)}
	defer reader.Close()
	if err:=reader.VerifyAll();err!=nil{t.Fatal(err)}
	foundCatalog,foundPayload,foundSig:=false,false,false
	for _,file:=range reader.Files(){
		switch{
		case file.Path=="catalog/catalog.json":foundCatalog=true
		case strings.HasPrefix(file.Path,"payloads/example/"):foundPayload=true
		case file.Path=="catalog/catalog.sig":foundSig=true
		}
	}
	if !foundCatalog||!foundPayload||!foundSig{
		t.Fatalf("SFX missing essential entries: %v",reader.Files())
	}
	if _,err:=BuildSFX(BuildOptions{Root:root,Destination:dest,PrivateKeyPath:private});err==nil{
		t.Fatal("existing SFX was overwritten")
	}
}
