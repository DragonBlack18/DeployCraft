package runner

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestOutputTailBoundedForLargeAndConcurrentWrites(t *testing.T) {
	b := &tailBuffer{}
	block := bytes.Repeat([]byte("x"), 1024*1024)
	n, err := b.Write(block)
	if err != nil || n != len(block) || len(b.String()) != tailLimit {
		t.Fatal("unbounded or incomplete writer")
	}
	_, _ = b.Write([]byte("end"))
	if !strings.HasSuffix(b.String(), "end") {
		t.Fatal("lost final output")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				_, _ = b.Write([]byte("abc"))
				_ = b.String()
			}
		}()
	}
	wg.Wait()
	if len(b.String()) > tailLimit {
		t.Fatal("tail grew beyond limit")
	}
}
