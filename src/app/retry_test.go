package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/domain"
	"github.com/DragonBlack18/DeployCraft/src/journal"
)

type sequenceProcess struct {
	codes []int
	marker string
	starts int
}
func (p *sequenceProcess) Start(command domain.Command)(domain.Running,error){
	p.starts++
	if !strings.HasSuffix(strings.ToLower(command.Exe),".exe") {
		return nil,errors.New("unexpected non-EXE command")
	}
	i:=p.starts-1
	if i>=len(p.codes){return nil,errors.New("too many process starts")}
	return &fakeRun{target:p.marker,install:p.codes[i]==0,exit:p.codes[i]},nil
}
func TestTransientCodeRetriesThenDetectsRealInstall(t *testing.T){
	e,f,_:=newFixture(t)
	seq:=&sequenceProcess{codes:[]int{1618,1618,0},marker:f.target}
	e.Process=seq
	p:=e.Catalog.Packages["example"]
	p.Install.RetryCodes=[]int{1618}
	e.Catalog.Packages["example"]=p
	var delays []time.Duration
	e.retryWait=func(ctx context.Context,d time.Duration)error{
		delays=append(delays,d)
		return nil
	}
	report,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	if report.ExitCode!=0 || report.Results[0].Outcome!=domain.OutcomeInstalled || seq.starts!=3 {
		t.Fatalf("unexpected retry outcome: report=%+v starts=%d",report,seq.starts)
	}
	if len(delays)!=2||delays[0]!=15*time.Second||delays[1]!=30*time.Second {
		t.Fatalf("invalid backoffs: %v",delays)
	}
	// The run must retain its journal and report, including the final attempt.
	if _,err:=os.Stat(filepath.Join(e.StateRoot,"logs",report.RunID,"report.json"));err!=nil{t.Fatal(err)}
	_ = journal.New(e.StateRoot)
}
func TestTransientRetryExhaustionNeverSucceeds(t *testing.T){
	e,f,_:=newFixture(t)
	seq:=&sequenceProcess{codes:[]int{1618,1618,1618,1618},marker:f.target}
	e.Process=seq
	p:=e.Catalog.Packages["example"]
	p.Install.RetryCodes=[]int{1618}
	e.Catalog.Packages["example"]=p
	delays:=[]time.Duration{}
	e.retryWait=func(_ context.Context,d time.Duration)error{
		delays=append(delays,d);return nil
	}
	report,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	result:=report.Results[0]
	if report.ExitCode!=domain.ExitPartial || result.Code!="E-RUN-003" ||
		result.Outcome!=domain.OutcomeFailed || result.ExitCode==nil||*result.ExitCode!=1618||seq.starts!=4 {
		t.Fatalf("retry exhaustion incorrectly succeeded: %+v starts=%d",result,seq.starts)
	}
	want:=[]time.Duration{15*time.Second,30*time.Second,60*time.Second}
	if len(delays)!=len(want){t.Fatalf("unexpected number of retry delays %v",delays)}
	for i:=range want{if delays[i]!=want[i]{t.Fatalf("backoff %d=%s want=%s",i,delays[i],want[i])}}
}
func TestUnlistedInstallerCodeNeverRetries(t *testing.T){
	e,f,_:=newFixture(t)
	seq:=&sequenceProcess{codes:[]int{1603,0},marker:f.target}
	e.Process=seq
	p:=e.Catalog.Packages["example"]
	p.Install.RetryCodes=[]int{1618}
	e.Catalog.Packages["example"]=p
	e.retryWait=func(_ context.Context,_ time.Duration)error{
		t.Fatal("unexpected retry for code 1603")
		return nil
	}
	report,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	if seq.starts!=1||report.Results[0].Code!="E-RUN-003" {
		t.Fatalf("unlisted code retried: report=%+v starts=%d",report,seq.starts)
	}
}
func TestTransientWaitDetectsExternalInstallInsteadOfReinstalling(t *testing.T){
	e,f,_:=newFixture(t)
	seq:=&sequenceProcess{codes:[]int{1618,0},marker:f.target}
	e.Process=seq
	p:=e.Catalog.Packages["example"]
	p.Install.RetryCodes=[]int{1618}
	e.Catalog.Packages["example"]=p
	e.retryWait=func(_ context.Context,_ time.Duration)error {
		return os.WriteFile(f.target,[]byte("already-installed"),0600)
	}
	report,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	if seq.starts!=1||report.ExitCode!=0||report.Results[0].Outcome!=domain.OutcomeInstalled{
		t.Fatalf("blind retry after installed: %+v starts=%d",report,seq.starts)
	}
}
func TestRetryCancelStopsFurtherStarts(t *testing.T){
	e,f,_:=newFixture(t)
	seq:=&sequenceProcess{codes:[]int{1618,0},marker:f.target}
	e.Process=seq
	p:=e.Catalog.Packages["example"]
	p.Install.RetryCodes=[]int{1618}
	e.Catalog.Packages["example"]=p
	e.retryWait=func(_ context.Context,_ time.Duration)error{return context.Canceled}
	report,err:=e.InstallOne(context.Background(),"example",labOptions())
	if err!=nil{t.Fatal(err)}
	if seq.starts!=1||report.Results[0].Outcome!=domain.OutcomeFailed||report.Results[0].Code!="E-RUN-001"{
		t.Fatalf("retry ignored cancellation: %+v",report)
	}
}
func TestWaitRetryContextCancelledImmediately(t *testing.T){
	ctx,cancel:=context.WithCancel(context.Background())
	cancel()
	start:=time.Now()
	if err:=waitRetryContext(ctx,time.Hour);!errors.Is(err,context.Canceled){t.Fatalf("expected cancellation: %v",err)}
	if time.Since(start)>time.Second{t.Fatal("cancelled backoff waited unexpectedly")}
}
