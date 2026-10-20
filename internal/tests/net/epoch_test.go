//go:build sim

package behavior_test

import (
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/internal/tests/testsupport"
	simnet "github.com/neria-cloud/gosim/net"
)

func TestRestartReusesEndpointWithoutOldConnection(t *testing.T) {
	firstAccepted, secondAccepted := make(chan struct{}), make(chan struct{})
	inspect := make(chan struct{})
	result := make(chan string)
	ready := make(chan struct{})
	server := testsupport.StartConfigured(t, gosim.MachineConfig{Main: func() {
		listener, err := net.Listen("tcp", "10.50.0.1:9000")
		testsupport.Check(t, err)
		close(ready)
		old, err := listener.Accept()
		testsupport.Check(t, err)
		close(firstAccepted)
		fresh, err := listener.Accept()
		testsupport.Check(t, err)
		close(secondAccepted)
		<-inspect
		if _, err := old.Write([]byte("stale")); err == nil {
			t.Fatal("old connection remained writable")
		}
		testsupport.Check(t, old.Close())
		testsupport.Check(t, fresh.SetReadDeadline(time.Now().Add(2*time.Second)))
		// Keep the new connection alive past the old queued packet.
		time.Sleep(1500 * time.Millisecond)
		var data [5]byte
		_, err = io.ReadFull(fresh, data[:])
		testsupport.Check(t, err)
		result <- string(data[:])
		testsupport.Check(t, fresh.Close())
		select {}
	}}, netip.MustParseAddr("10.50.0.1"))
	<-ready
	addresses := make(chan string)
	sendOld, oldQueued := make(chan struct{}), make(chan struct{})
	client := testsupport.StartConfigured(t, gosim.MachineConfig{Main: func() {
		conn, err := net.Dial("tcp", "10.50.0.1:9000")
		testsupport.Check(t, err)
		addresses <- conn.LocalAddr().String()
		<-sendOld
		_, err = conn.Write([]byte("stale"))
		testsupport.Check(t, err)
		close(oldQueued)
		testsupport.Check(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
		var data [1]byte
		_, _ = conn.Read(data[:])
		t.Error("crashed blocked read returned")
	}}, netip.MustParseAddr("10.50.0.2"))
	<-firstAccepted
	before := <-addresses
	link := simnet.Link{
		From:      netip.MustParseAddr("10.50.0.2"),
		To:        netip.MustParseAddr("10.50.0.1"),
		Direction: simnet.Forward,
	}
	testsupport.Check(
		t,
		simnet.SetLinkPolicy(
			link,
			simnet.LinkPolicy{Latency: &simnet.LatencyConfig{Min: time.Second, Max: time.Second, Lambda: 1}},
		),
	)
	close(sendOld)
	<-oldQueued
	time.Sleep(time.Millisecond)
	testsupport.Check(t, client.Crash())
	testsupport.Check(t, simnet.ResetLinkPolicy(link))
	testsupport.Check(t, client.SetMain(func() {
		conn, err := net.Dial("tcp", "10.50.0.1:9000")
		testsupport.Check(t, err)
		addresses <- conn.LocalAddr().String()
		_, err = conn.Write([]byte("fresh"))
		testsupport.Check(t, err)
		select {}
	}))
	testsupport.Check(t, client.Restart())
	<-secondAccepted
	after := <-addresses
	if before != after {
		t.Fatalf("fixture did not reuse endpoint: %s %s", before, after)
	}
	close(inspect)
	if data := <-result; data != "fresh" {
		t.Fatalf("recycled connection received old data: %q", data)
	}
	// Drain past the queued old packet and the old read deadline. Neither may
	// touch the replacement connection or resume the canceled application.
	time.Sleep(2 * time.Second)
	testsupport.Check(t, client.Crash())
	testsupport.Check(t, server.Crash())
}
