//go:build sim

package behavior_test

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/gosimlog"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
	"github.com/neria-cloud/gosim/rt/runtime"
)

func TestLogSLog(t *testing.T) {
	slog.Info("hello 10")
}

func TestLogMachineGoroutineTime(t *testing.T) {
	done := make(chan struct{})

	log.Printf("hi %s", "there")
	go func() {
		testsupport.StartConfigured(t, gosim.MachineConfig{
			Label: "inside",
			Main: func() {
				time.Sleep(10 * time.Second)
				log.Print("hey")
				go func() {
					time.Sleep(10 * time.Second)
					log.Print("propagated")
					slog.Warn("foo", "bar", "baz", "counter", 20)
					close(done)
				}()
				<-done
			},
		})
	}()

	<-done
}

func TestLogStdoutStderr(t *testing.T) {
	os.Stdout.WriteString("hello\n")
	os.Stderr.WriteString("goodbye\n")
	log.Println("same goroutine log")
	slog.Info("same goroutine slog")
}

func TestLogForPrettyTest(t *testing.T) {
	if os.Getenv("TESTINGFAIL") != "1" {
		t.Skip()
	}

	slog.Info("hello info", "foo", "bar")
	start := time.Now()
	time.Sleep(20 * time.Second)

	m := testsupport.Start(t, func() {
		slog.Info("warn", "ok", "now", "delay", time.Since(start))
		log.Println("before")
		panic("help")
	})
	testsupport.Wait(t, m)
	log.Println("never")
}

func init() {
	// logs during init will print to stdout/stderr bypassing the gosim handler
	if os.Getenv("LOGDURINGINIT") == "1" {
		fmt.Println("hello")
		slog.Info("help")
	}
}

func TestLogDuringInit(t *testing.T) {
	// logs in above init() should print once for main

	// logs should print for this machine also
	testsupport.Wait(t, testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "logm",
		Main:  func() {},
	}))
}

func c(z *slog.Logger) {
	slog.Info("hello from slog")
	z.Info("hello from logger")
}

func b(z *slog.Logger) {
	c(z)
}

func a(z *slog.Logger, count int) {
	if count == 0 {
		b(z)
	} else {
		a(z, count-1)
	}
}

func TestLogTraceStacks(t *testing.T) {
	log.Println("hello from log")

	z := slog.Default().With("component", "stack-test")

	// print some logs with interesting stacks
	a(z, 0)
	a(z, 3)
}

func testBacktraceForB() {
	select {}
}

func testBacktraceForA() {
	testBacktraceForB()
}

func TestLogBacktraceFor(t *testing.T) {
	g0 := gosimruntime.GetGoroutine()
	var g1 int

	go func() {
		g1 = gosimruntime.GetGoroutine()
		gosimruntime.TestRaceToken.Release()
		testBacktraceForA()
	}()

	time.Sleep(time.Second)
	gosimruntime.TestRaceToken.Acquire()

	var pcs [128]uintptr
	n := gosimruntime.GetStacktraceFor(g0, pcs[:])
	slog.Info("g0", gosimlog.StackFor(pcs[:n], 0))

	n = gosimruntime.GetStacktraceFor(g1, pcs[:])
	slog.Info("g1", gosimlog.StackFor(pcs[:n], 0))
}
