package journal

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)
func TestAppendAndSnapshots(t *testing.T){
	root:=t.TempDir()
	s:=New(root)
	id,err:=NewRunID();if err!=nil{t.Fatal(err)}
	for _,event:=range []string{"run.started","step.started","step.finished","run.finished"} {
		if err:=s.Append(Event{RunID:id,Event:event});err!=nil{t.Fatal(err)}
	}
	f,err:=os.Open(filepath.Join(root,"state","journal.jsonl"));if err!=nil{t.Fatal(err)}
	defer f.Close()
	scanner:=bufio.NewScanner(f)
	n:=0
	for scanner.Scan(){
		var ev Event
		if err:=json.Unmarshal(scanner.Bytes(),&ev);err!=nil{t.Fatal(err)}
		if ev.RunID!=id {t.Fatalf("runId changed: %s",ev.RunID)}
		n++
	}
	if err:=scanner.Err();err!=nil{t.Fatal(err)}
	if n!=4{t.Fatalf("expected 4 journal events, got %d",n)}
	if err:=s.WriteSnapshot(Snapshot{RunID:id,Status:"done",Packages:map[string]string{"example":"installed"}});err!=nil{t.Fatal(err)}
	if err:=s.WriteReport(id,map[string]any{"exitCode":0});err!=nil{t.Fatal(err)}
	for _,path:=range []string{filepath.Join(root,"state","state.json"),filepath.Join(root,"logs",id,"report.json")} {
		if _,err:=os.Stat(path);err!=nil{t.Fatal(err)}
	}
}
func TestJournalRejectsUnidentifiedEvents(t *testing.T){
	s:=New(t.TempDir())
	if err:=s.Append(Event{Event:"step.started"});err==nil{t.Fatal("missing runId accepted")}
}
