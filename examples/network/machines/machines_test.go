//go:build sim

package machines_test

import (
	"bufio"
	"fmt"
	"net"
	"net/netip"
	"os"
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

func TestMachineSyscallMonitor(t *testing.T) {
	ready, run := make(chan struct{}), make(chan struct{})
	m, err := gosim.NewMachine(gosim.MachineConfig{Label: "files", Main: func() {
		close(ready)
		<-run // The controller starts counting after package initialization.
		file, err := os.OpenFile("monitored.txt", os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString("example"); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		data := make([]byte, len("example"))
		if n, err := file.Read(data); err != nil || n != len(data) || string(data) != "example" {
			t.Fatalf("read %q: %v", data, err)
		}
		if err := file.Sync(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	monitor := m.SyscallsMonitor()
	available, err := gosim.CountableSyscalls()
	if err != nil {
		t.Fatal(err)
	}
	groups := make(map[string]gosim.SyscallGroup, len(available))
	for _, info := range available {
		groups[info.Name] = info.Group
	}
	if groups["read"] != gosim.SyscallGroupRT || groups["openat"] != gosim.SyscallGroupFS {
		t.Fatal("syscall groups", groups)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	<-ready
	if err := monitor.Start(); err != nil {
		t.Fatal(err)
	}
	close(run)
	if _, err := m.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := monitor.Stop(); err != nil {
		t.Fatal(err)
	}
	counts, err := monitor.Counts()
	if err != nil {
		t.Fatal(err)
	}
	stats, err := monitor.Stats()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"read", "write", "close", "fsync"} {
		stat := stats[name]
		if counts[name] == 0 || stat.Count != counts[name] || stat.ByDescriptor[gosim.DescriptorGroupFS] != stat.Count || stat.Group != groups[name] {
			t.Fatalf("%s: count=%d stats=%+v", name, counts[name], stat)
		}
	}
	if stats["openat"].ByDescriptor[gosim.DescriptorGroupNone] == 0 {
		t.Fatal("openat has no existing input descriptor")
	}
	if err := monitor.Reset(); err != nil {
		t.Fatal(err)
	}
	cleared, err := monitor.Counts()
	if err != nil || len(cleared) != 0 {
		t.Fatal("reset", cleared, err)
	}
}
