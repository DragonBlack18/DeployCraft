package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

type testRegistry struct{}
func (testRegistry) ReadString(key,value string)(string,bool,error){return "",false,nil}
func (testRegistry) ListSubKeys(key string)([]string,error){return nil,nil}
type testPaths struct{}
func (testPaths) Expand(text string)string{return text}
func (testPaths) ProgramData()string{return ""}
func (testPaths) Temp()string{return os.TempDir()}
type fakeRun struct {target string;install bool;exit int}
func (r *fakeRun)Wait(ctx context.Context)(int,error){
	if r.install{if err:=os.WriteFile(r.target,[]byte("installed"),0600);err!=nil{return -1,err}}
	return r.exit,nil
}
func (r *fakeRun)Kill()error{return nil}
type fakeProcess struct {target string;install bool;exit int;starts int}
func (f *fakeProcess)Start(command domain.Command)(domain.Running,error){
	f.starts++
	if !strings.HasSuffix(strings.ToLower(command.Exe),".exe"){return nil,errors.New("expected copied EXE")}
	return &fakeRun{target:f.target,install:f.install,exit:f.exit},nil
}
func newFixture(t *testing.T)(*InstallEngine,*fakeProcess,string) {
	t.Helper()
	kit:=t.TempDir()
	src:=filepath.Join(kit,"payloads","example")
	if err:=os.MkdirAll(src,0700);err!=nil{t.Fatal(err)}
	data:=[]byte("fixture only, never executed")
	payload:=filepath.Join(src,"setup.exe")
	if err:=os.WriteFile(payload,data,0600);err!=nil{t.Fatal(err)}
	h:=sha256.Sum256(data)
	installed:=filepath.Join(t.TempDir(),"installed.txt")
	p:=domain.Package{
		Schema:1,ID:"example",Name:"Example",Version:"1.0",
		Platform:domain.Platform{OS:"windows",Arch:[]string{"x64"}},
		Payload:domain.Payload{Kind:"file",Path:"payloads/example/setup.exe",SHA256:hex.EncodeToString(h[:]),Size:int64(len(data))},
		Install:domain.Install{Driver:"exe",TimeoutSec:2,SuccessCodes:[]int{0}},
		Detect:domain.Detect{Match:"all",Rules:[]domain.DetectRule{{Type:"file",Path:installed}}},
	}
	fake:=&fakeProcess{target:installed,install:true}
	engine:=&InstallEngine{
		Catalog:domain.Catalog{Packages:map[string]domain.Package{"example":p}},
		KitRoot:kit,StateRoot:t.TempDir(),Registry:testRegistry{},Paths:testPaths{},
		Process:fake,IsAdmin:func()(bool,error){return true,nil},
		postCheckTimeout:25*time.Millisecond,labPlatformOverride:true,
	}
	return engine,fake,payload
}
func labOptions()InstallOptions{return InstallOptions{Confirmed:true,LabUnsignedCatalog:true}}
func TestInstallAndSecondRunSkips(t *testing.T){
	e,f,_:=newFixture(t)
	first,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	if len(first.Results)!=1 || first.Results[0].Outcome!=domain.OutcomeInstalled{t.Fatalf("first: %+v",first)}
	second,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	if second.Results[0].Outcome!=domain.OutcomeSkipped{t.Fatalf("second: %+v",second)}
	if f.starts!=1{t.Fatalf("installer started %d times",f.starts)}
}
func TestInstallExitCodeZeroWithoutDetectionFails(t *testing.T){
	e,f,_:=newFixture(t)
	f.install=false
	report,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	if report.Results[0].Outcome!=domain.OutcomeFailed || report.Results[0].Code!="E-RUN-004"{
		t.Fatalf("must reject success without post-detection: %+v",report)
	}
}
func TestInstallRejectsHashTamperBeforeProcess(t *testing.T){
	e,f,payload:=newFixture(t)
	if err:=os.WriteFile(payload,[]byte("changed file with different size"),0600);err!=nil{t.Fatal(err)}
	report,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	if report.Results[0].Reason!=domain.ReasonIntegrity || f.starts!=0 {
		t.Fatalf("integrity bypass: %+v starts=%d",report,f.starts)
	}
}
func TestLabFlagIsMandatory(t *testing.T){
	e,f,_:=newFixture(t)
	_,err:=e.InstallOne(context.Background(),"example",InstallOptions{Confirmed:true})
	var pre *PreflightError
	if !errors.As(err,&pre) || pre.Code!="E-VER-004" || f.starts!=0 {t.Fatalf("unsafe unsigned install: %v",err)}
}
func TestDryRunNeverCreatesPayloadProcess(t *testing.T){
	e,f,_:=newFixture(t)
	report,err:=e.InstallOne(context.Background(),"example",InstallOptions{DryRun:true,LabUnsignedCatalog:true})
	if err!=nil{t.Fatal(err)}
	if f.starts!=0 || report.Results[0].Outcome!=domain.OutcomeSkipped{t.Fatalf("dry run executed: %+v",report)}
}
func TestUnknownMustNotBecomeAbsent(t *testing.T){
	e,f,_:=newFixture(t)
	p:=e.Catalog.Packages["example"]
	p.Detect.Rules=[]domain.DetectRule{{Type:"command",Command:"test",Matches:"test"}}
	e.Catalog.Packages["example"]=p
	_,err:=e.InstallOne(context.Background(),"example",labOptions())
	var pre *PreflightError
	if !errors.As(err,&pre) || pre.Code!="E-PRE-006" || f.starts!=0{t.Fatalf("unknown treated as absent: %v",err)}
}
