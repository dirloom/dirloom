// Command helper is the child process of the F9 and F15 facts.
//
//	helper signal-wait
//	helper blocked-writer [-size n] [-grace d] [-self-interrupt d]
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: helper signal-wait | blocked-writer")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "signal-wait":
		signalWait()
	case "blocked-writer":
		blockedWriter(os.Args[2:])
	default:
		os.Exit(2)
	}
}

func signalWait() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	fmt.Println("READY")
	select {
	case sig := <-ch:
		fmt.Printf("GOT %v\n", sig)
		os.Exit(0)
	case <-time.After(15 * time.Second):
		fmt.Println("TIMEOUT")
		os.Exit(3)
	}
}

// blockedWriter mimics the planned engine: one writer goroutine, a signal
// handler, and an output grace period after which the process exits 5.
func blockedWriter(args []string) {
	fs := flag.NewFlagSet("blocked-writer", flag.ExitOnError)
	size := fs.Int("size", 4<<20, "bytes of NDJSON to write")
	grace := fs.Duration("grace", 2*time.Second, "output grace period")
	self := fs.Duration("self-interrupt", 0, "interrupt itself after this delay once blocked (0: wait for a signal)")
	_ = fs.Parse(args)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	line := []byte(`{"schemaVersion":1,"event":"node.added","generation":1,"path":"src/features/payments/module-000000","after":{"kind":"directory"}}` + "\n")
	payload := bytes.Repeat(line, *size/len(line)+1)
	done := make(chan error, 1)
	go func() {
		_, err := os.Stdout.Write(payload)
		done <- err
	}()
	fmt.Fprintln(os.Stderr, "STARTED")

	blocked := time.After(500 * time.Millisecond)
	var selfC <-chan time.Time
	for {
		select {
		case err := <-done:
			fmt.Fprintf(os.Stderr, "WRITE-DONE err=%v\n", err)
			os.Exit(0)
		case <-blocked:
			fmt.Fprintln(os.Stderr, "BLOCKED")
			if *self > 0 {
				selfC = time.After(*self)
			}
		case sig := <-ch:
			stop(fmt.Sprint(sig), done, *grace)
		case <-selfC:
			stop("self", done, *grace)
		}
	}
}

func stop(reason string, done <-chan error, grace time.Duration) {
	fmt.Fprintf(os.Stderr, "INTERRUPT %s\n", reason)
	select {
	case err := <-done:
		fmt.Fprintf(os.Stderr, "STOPPED-CLEANLY err=%v\n", err)
		os.Exit(0)
	case <-time.After(grace):
		fmt.Fprintln(os.Stderr, "GRACE-EXPIRED")
		os.Exit(5)
	}
}
