// fake-installer is a test fixture, not a real third-party installer.
// It is never embedded into the motor or shipped in a production kit.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	exit := flag.Int("exit", 0, "exit code")
	sleep := flag.Duration("sleep", 0, "artificial delay")
	marker := flag.String("write-file", "", "write an explicit test marker")
	flag.Parse()
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
