// fake-installer is a test fixture, not a real third-party installer.
// It is never embedded into the motor or shipped in a production kit.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	exit := flag.Int("exit", 0, "exit code")
	sleep := flag.Duration("sleep", 0, "artificial delay")
	marker := flag.String("write-file", "", "write an explicit test marker")
	attemptFile := flag.String("attempt-file", "", "test-only file tracking process launches")
	retryUntil := flag.Int("retry-until", 0, "return transient exit while attempt count is <= this value")
	retryCode := flag.Int("retry-code", 1618, "transient Windows exit code")
	flag.Parse()
	if *attemptFile != "" {
		count := 0
		if bytes, err := os.ReadFile(*attemptFile); err == nil {
			count, err = strconv.Atoi(strings.TrimSpace(string(bytes)))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		count++
		if err := os.WriteFile(*attemptFile, []byte(strconv.Itoa(count)), 0600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if count <= *retryUntil {
			os.Exit(*retryCode)
		}
	}
	if *sleep > 0 {
		time.Sleep(*sleep)
	}
	if *marker != "" {
		if err := os.WriteFile(*marker, []byte("fake-installed"), 0600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	os.Exit(*exit)
}
