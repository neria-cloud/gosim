// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found at https://go.googlesource.com/go/+/refs/heads/master/LICENSE.

package race_test

import (
	"io"
	"net"
	"testing"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
)

// from go/src/runtime/race/testdata/io_test.go

// XXX: broken
/*
func TestNoRaceIOFile(t *testing.T) {
	if !gosimruntime.IsDetgo() {
		setupRealDisk(s)
	}
	x := 0
	path := "." // t.TempDir()
	fname := filepath.Join(path, "data")
	go func() {
		x = 42
		f, _ := os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		f.Write([]byte("done"))
		f.Close()
	}()
	for {
		f, err := os.OpenFile(fname, os.O_RDONLY, 0)
		if err != nil {
			time.Sleep(1e6)
			continue
		}
		buf := make([]byte, 100)
		count, err := f.Read(buf)
		if count == 0 {
			time.Sleep(1e6)
			continue
		}
		break
	}
	_ = x
}
*/

// TCP request/response work uses an explicit completion edge before the
// controller reads shared state, under both the native and simulated race oracle.
func TestNoRaceIOTCP(t *testing.T) {
	address := "127.0.0.1:0"
	if gosim.IsSim() {
		ip, err := simnet.Address(gosim.CurrentMachine())
		if err != nil {
			t.Fatal(err)
		}
		address = net.JoinHostPort(ip.String(), "18080")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	completed := make(chan error, 1)
	handled := 0
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			completed <- err
			return
		}
		defer conn.Close()
		for range 2 {
			var request [1]byte
			if _, err := io.ReadFull(conn, request[:]); err != nil {
				completed <- err
				return
			}
			handled++
			if _, err := conn.Write(request[:]); err != nil {
				completed <- err
				return
			}
		}
		completed <- nil
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for range 2 {
		if _, err := conn.Write([]byte{42}); err != nil {
			t.Fatal(err)
		}
		var response [1]byte
		if _, err := io.ReadFull(conn, response[:]); err != nil {
			t.Fatal(err)
		}
		if response[0] != 42 {
			t.Fatal(response)
		}
	}
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	if handled != 2 {
		t.Fatal(handled)
	}
}
