//go:build sim

package machines_test

import (
	"bufio"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
)

var count int

func server() {
	listener, err := net.Listen("tcp", "10.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		count++
		if _, err := fmt.Fprintf(conn, "hello from the server! request: %d\n", count); err != nil {
			panic(err)
		}
		if err := conn.Close(); err != nil {
			panic(err)
		}
	}
}

func request(t *testing.T, want int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "10.0.0.1:8080", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	body, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if body != fmt.Sprintf("hello from the server! request: %d\n", want) {
		t.Fatal(body)
	}
	t.Log(body)
}

func TestMachines(t *testing.T) {
	m, err := gosim.NewMachine(gosim.MachineConfig{Label: "server", Main: server})
	if err != nil {
		t.Fatal(err)
	}
	serverAddress := netip.MustParseAddr("10.0.0.1")
	if err := simnet.ConfigureMachine(m, simnet.MachineConfig{Addr: serverAddress}); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	request(t, 1)
	request(t, 2)
	if err := m.Crash(); err != nil {
		t.Fatal(err)
	}
	if err := m.Restart(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	request(t, 1)
	clientAddress, err := simnet.Address(gosim.CurrentMachine())
	if err != nil {
		t.Fatal(err)
	}
	delay := simnet.LatencyConfig{Min: 200 * time.Millisecond, Max: 200 * time.Millisecond, Lambda: 5}
	if err := simnet.SetLinkPolicy(simnet.Link{From: serverAddress, To: clientAddress}, simnet.LinkPolicy{Latency: &delay}); err != nil {
		t.Fatal(err)
	}
	request(t, 2)
}
