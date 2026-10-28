// Package runner executes explicit programs directly, never using a shell.
// Windows Job Object supervision is scheduled for Phase 4.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

type nativeProcess struct{}
type running struct {
	cmd *exec.Cmd
	tail *tailBuffer
}
type tailBuffer struct {
	mu sync.Mutex
	buf []byte
}
func (t *tailBuffer) Write(p []byte)(int,error){
	t.mu.Lock();defer t.mu.Unlock()
	t.buf=append(t.buf,p...)
	if len(t.buf)>16384{t.buf=append([]byte(nil),t.buf[len(t.buf)-16384:]...)}
	return len(p),nil
}
func (t *tailBuffer) String() string{
	t.mu.Lock();defer t.mu.Unlock()
	return string(bytes.ToValidUTF8(t.buf,[]byte("?")))
}
func New() domain.Process{return nativeProcess{}}
func (nativeProcess) Start(command domain.Command)(domain.Running,error){
	if command.Exe=="" || command.RawCmdLine!=""{
		return nil,fmt.Errorf("E-DRV-002: comando vazio ou rawCmdLine indisponível")
	}
	cmd:=exec.Command(command.Exe,command.Args...)
	cmd.Dir=command.Cwd
	if len(command.Env)>0 {
		cmd.Env=append(os.Environ(),command.Env...)
	}
	tail:=&tailBuffer{}
	cmd.Stdout=tail
	cmd.Stderr=tail
	cmd.WaitDelay=3*time.Second
	if err:=cmd.Start();err!=nil{return nil,fmt.Errorf("E-RUN-002: falha ao iniciar processo: %w",err)}
	return &running{cmd:cmd,tail:tail},nil
}
func (r *running)Tail()string{return r.tail.String()}
func (r *running)Kill()error{
	if r.cmd.Process==nil{return errors.New("processo não iniciado")}
	return r.cmd.Process.Kill()
}
func (r *running)Wait(ctx context.Context)(int,error){
	done:=make(chan error,1)
	go func(){done<-r.cmd.Wait()}()
	select{
	case err:=<-done:
		if err==nil{return 0,nil}
		var exitErr *exec.ExitError
		if errors.As(err,&exitErr){return exitErr.ExitCode(),nil}
		return -1,err
	case <-ctx.Done():
		_=r.Kill()
		<-done
		return -1,ctx.Err()
	}
}
