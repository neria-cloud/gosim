//go:build sim

package nemesis_test

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/nemesis"
	simnet "github.com/neria-cloud/gosim/net"
)

func makeAddresses() []string { return []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} }
func request(addr string) error {
	conn, err := net.DialTimeout("tcp", addr+":8080", time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	data := make([]byte, len("hello there"))
	_, err = io.ReadFull(conn, data)
	if err != nil {
		return err
	}
	if string(data) != "hello there" {
		return fmt.Errorf("unexpected response %q", data)
	}
	return nil
}

func pingerMain() {
	address, err := simnet.Address(gosim.CurrentMachine())
	if err != nil {
		panic(err)
	}
	listener, err := net.Listen("tcp", address.String()+":8080")
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			if _, err := conn.Write([]byte("hello there")); err != nil {
				log.Print(err)
			}
			if err := conn.Close(); err != nil {
				log.Print(err)
			}
		}
	}()
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		for _, other := range makeAddresses() {
			if other != address.String() {
				log.Printf("pinging %s: %v", other, request(other))
			}
		}
	}
}

func TestPartition(t *testing.T) {
	addresses := makeAddresses()
	var machines []gosim.Machine
	for _, address := range addresses {
		m, err := gosim.NewMachine(gosim.MachineConfig{Label: address, Main: pingerMain})
		if err != nil {
			t.Fatal(err)
		}
		if err := simnet.ConfigureMachine(m, simnet.MachineConfig{Addr: netip.MustParseAddr(address)}); err != nil {
			t.Fatal(err)
		}
		if err := m.Start(); err != nil {
			t.Fatal(err)
		}
		machines = append(machines, m)
	}
	nemesis.Sequence(
		nemesis.Sleep{Duration: 10 * time.Second},
		nemesis.PartitionMachines{Addresses: addresses, Duration: 10 * time.Second},
		nemesis.Sleep{Duration: 10 * time.Second},
		nemesis.RestartRandomly{Machines: machines, Downtime: 10 * time.Second},
		nemesis.Sleep{Duration: 10 * time.Second},
	).Run()
	for _, address := range addresses {
		if err := request(address); err != nil {
			t.Fatalf("recovered %s: %v", address, err)
		}
	}
}
