//go:build sim

package behavior_test

import (
	"errors"
	"io"
	"log"
	"net"
	"net/netip"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
	"github.com/neria-cloud/gosim/rt/runtime"
)

// EchoServer is a small TCP server that listens on 10.0.0.1:8080 and echoes
// all incoming bytes.
func EchoServer() {
	l, err := net.Listen("tcp", "10.0.0.1:8080") // XXX: need to specify IP? wtf?
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			defer conn.Close()
			for {
				var buf [1024]byte
				n, err := conn.Read(buf[:])
				if err != nil {
					log.Println(err)
					return
				}
				if _, err := conn.Write(buf[:n]); err != nil {
					log.Println(err)
					return
				}
			}
		}()
	}
}

func TestCrashServerTCPConnBreaks(t *testing.T) {
	// run echo server
	m := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "a",

		Main: EchoServer,
	}, netip.AddrFrom4([4]byte{10, 0, 0, 1}))

	// let it start listening
	time.Sleep(time.Second)

	// dial it and see it echo
	conn, err := net.Dial("tcp", "10.0.0.1:8080")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	var buf [5]byte
	if _, err := io.ReadFull(conn, buf[:]); err != nil {
		t.Fatal(err)
	}
	if string(buf[:]) != "hello" {
		t.Fatal(string(buf[:]))
	}

	// let it sit and cash it
	time.Sleep(time.Second)
	testsupport.Check(t, m.Crash())

	// XXX: test without any data sent?

	// XXX: should have network behavior be configurable: host not responding vs rst vs ...

	// then, expect read to eventually fail after write never gets acked
	_, err = conn.Read(buf[:])
	if err == nil {
		t.Error("expected error")
	}
	if !errors.Is(err, syscall.EPIPE) {
		t.Errorf("expected syscall.EPIPE, got %s", err)
	}
}

// EchoClient is a small TCP client that connects to on 10.0.0.1:8080 and writes
// "hello" once and expects it come back.
func EchoClient() {
	conn, err := net.Dial("tcp", "10.0.0.1:8080")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := conn.Write([]byte("hello")); err != nil {
		log.Fatal(err)
	}
	var buf [5]byte
	if _, err := io.ReadFull(conn, buf[:]); err != nil {
		log.Fatal(err)
	}
	if string(buf[:]) != "hello" {
		log.Fatal(string(buf[:]))
	}
}

func TestCrashClientTCPConnBreaks(t *testing.T) {
	// run echo server
	var done atomic.Bool
	gosimruntime.TestRaceToken.Release()
	testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "a",

		Main: func() {
			gosimruntime.TestRaceToken.Acquire()

			l, err := net.Listen("tcp", "10.0.0.1:8080") // XXX: need to specify IP? wtf?
			if err != nil {
				log.Fatal(err)
			}

			conn, err := l.Accept()
			if err != nil {
				log.Fatal(err)
			}
			var buf [5]byte
			n, err := io.ReadFull(conn, buf[:])
			if err != nil {
				log.Println(err)
				return
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				log.Println(err)
				return
			}

			// then, expect (eventually) an error because the client crashed and the network disconnected
			_, err = conn.Read(buf[:])
			if err == nil {
				t.Error("expected error")
			}
			if !errors.Is(err, syscall.EPIPE) {
				t.Errorf("expected syscall.EPIPE, got %s", err)
			}
			done.Store(true)
		},
	}, netip.AddrFrom4([4]byte{10, 0, 0, 1}))

	// let it start listening
	time.Sleep(time.Second)

	client := testsupport.Start(t, EchoClient)

	// let it connect and do the echo
	time.Sleep(time.Second)

	// then crash the client
	testsupport.Check(t, client.Crash())

	// wait for the server to notice
	time.Sleep(time.Minute)

	if !done.Load() {
		t.Error("expected done")
	}
}

func checkEcho(t *testing.T) {
	t.Helper()
	// dial it and see it echo
	conn, err := net.Dial("tcp", "10.0.0.1:8080")
	defer conn.Close()

	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	var buf [5]byte
	if _, err := io.ReadFull(conn, buf[:]); err != nil {
		t.Fatal(err)
	}
	if string(buf[:]) != "hello" {
		t.Fatal(string(buf[:]))
	}
}

func TestCrashRestartNetwork(t *testing.T) {
	// run echo server
	m := testsupport.StartConfigured(t, gosim.MachineConfig{
		Label: "a",

		Main: EchoServer,
	}, netip.AddrFrom4([4]byte{10, 0, 0, 1}))

	// let it start listening
	time.Sleep(time.Second)

	// check we can dial
	conn, err := net.Dial("tcp", "10.0.0.1:8080")
	if err != nil {
		t.Fatalf("expected dial success, got %v", err)
	}
	conn.Close()

	// check echo works
	checkEcho(t)

	// crash
	testsupport.Check(t, m.Crash())

	// check dialing fails
	_, err = net.Dial("tcp", "10.0.0.1:8080")
	if err == nil {
		t.Fatalf("expected dial failure, got %v", err)
	}

	// restart
	testsupport.Check(t, m.Restart()) // still runs the old EchoServer

	time.Sleep(time.Second)

	// check we can dial again
	conn, err = net.Dial("tcp", "10.0.0.1:8080")
	if err != nil {
		t.Fatalf("expected dial success, got %v", err)
	}
	conn.Close()

	// check echo works again
	checkEcho(t)

	// XXX: test what happens with in-flight packets
}
