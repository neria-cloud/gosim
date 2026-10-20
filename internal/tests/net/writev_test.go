//go:build sim

package behavior_test

import (
	"bytes"
	"io"
	"net"
	"net/netip"
	"testing"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
)

func TestNetBuffersWriteTo(t *testing.T) {
	ready := make(chan struct{})
	payload := bytes.Repeat([]byte("ordered"), 20000)
	server := testsupport.StartConfigured(t, gosim.MachineConfig{Label: "writev-server", Main: func() {
		listener, err := net.Listen("tcp", aAddr+":4321")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		close(ready)
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		got := make([]byte, len(payload))
		if _, err := io.ReadFull(conn, got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatal("scatter/gather write changed byte order")
		}
	}}, netip.MustParseAddr(aAddr))
	client := testsupport.StartConfigured(t, gosim.MachineConfig{Label: "writev-client", Main: func() {
		<-ready
		conn, err := net.Dial("tcp", aAddr+":4321")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		buffers := net.Buffers{nil, payload[:13], {}, payload[13:70000], payload[70000:]}
		n, err := buffers.WriteTo(conn)
		if err != nil || n != int64(len(payload)) {
			t.Fatalf("writev count=%d err=%v", n, err)
		}
		if len(buffers) != 0 {
			t.Fatal("buffers not consumed")
		}
	}}, netip.MustParseAddr(bAddr))
	testsupport.Wait(t, server)
	testsupport.Wait(t, client)
}
