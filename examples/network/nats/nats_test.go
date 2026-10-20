//go:build sim

package nats_test

import (
	"fmt"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/neria-cloud/gosim"
	simnet "github.com/neria-cloud/gosim/net"
	"github.com/neria-cloud/gosim/rt"
)

func TestPublisherReceiver(t *testing.T) {
	runPublisherReceiver(t, simnet.LatencyConfig{Min: time.Second, Max: 3 * time.Second, Lambda: 5})
}

func TestPublisherReceiverWithoutLatency(t *testing.T) {
	runPublisherReceiver(t, simnet.LatencyConfig{Lambda: 5})
}

func runPublisherReceiver(t *testing.T, linkLatency simnet.LatencyConfig) {
	t.Helper()
	const (
		url               = "nats://10.62.0.1:4222"
		subject           = "example.numbers"
		sendTime          = "Send-Time"
		count             = 10
		first             = 1000
		connectionTimeout = 5 * time.Second
		messageWait       = 10 * time.Second
	)
	check := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	check(rt.SetTimeout(time.Minute))
	config := simnet.DefaultConfig()
	config.ConnectionTimeout = connectionTimeout
	// Prepare subscriptions before injecting delay, so publication cannot race
	// subscription registration. Apply the scenario's latency afterward.
	config.Latency = simnet.LatencyConfig{Lambda: 5}
	check(simnet.Configure(config))
	start := func(label, address string, main func()) {
		m, err := gosim.NewMachine(gosim.MachineConfig{Label: label, Main: main})
		check(err)
		check(simnet.ConfigureMachine(m, simnet.MachineConfig{Addr: netip.MustParseAddr(address)}))
		check(m.Start())
		t.Cleanup(func() {
			if err := m.Stop(); err != nil {
				t.Error(err)
			}
			if err := m.Destroy(); err != nil {
				t.Error(err)
			}
		})
	}

	serverReady := make(chan struct{})
	shutdown := make(chan struct{})
	start("nats-server", "10.62.0.1", func() {
		s, err := server.NewServer(&server.Options{
			Host: "10.62.0.1", Port: 4222,
			NoSigs: true, NoLog: true, NoSystemAccount: true, WriteDeadline: connectionTimeout,
		})
		check(err)
		go s.Start()
		if !s.ReadyForConnections(connectionTimeout) {
			panic("embedded NATS server did not become ready")
		}
		defer s.Shutdown()
		close(serverReady)
		<-shutdown
	})
	<-serverReady

	connect := func(name string) *nats.Conn {
		conn, err := nats.Connect(url, nats.Name(name), nats.Timeout(connectionTimeout), nats.NoReconnect())
		check(err)
		return conn
	}
	subscribed, publisherReady := make(chan struct{}), make(chan struct{})
	begin := make(chan struct{})
	received, published := make(chan struct{}), make(chan struct{})
	start("receiver", "10.62.0.3", func() {
		conn := connect("receiver")
		defer conn.Close()
		sub, err := conn.SubscribeSync(subject)
		check(err)
		check(conn.FlushTimeout(connectionTimeout))
		close(subscribed)
		<-begin
		for sequence := first; sequence < first+count; sequence++ {
			msg, err := sub.NextMsg(messageWait)
			check(err)
			now := time.Now()
			sent, err := time.Parse(time.RFC3339Nano, msg.Header.Get(sendTime))
			check(err)
			value, err := strconv.Atoi(string(msg.Data))
			check(err)
			if value != sequence {
				panic(fmt.Sprintf("received %d; expected %d", value, sequence))
			}
			latency := now.Sub(sent)
			// Two network legs each contribute their minimum delay; buffering
			// and stream ordering can add delay beyond their maximum sum.
			if latency < 2*linkLatency.Min {
				panic(fmt.Sprintf("unexpected end-to-end latency: %dns", latency.Nanoseconds()))
			}
			fmt.Printf("RECEIVE payload=%d sent=%s received=%s latency=%dns\n",
				value, sent.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), latency.Nanoseconds())
		}
		close(received)
		<-shutdown
	})
	start("publisher", "10.62.0.2", func() {
		conn := connect("publisher")
		defer conn.Close()
		close(publisherReady)
		<-begin
		for sequence := first; sequence < first+count; sequence++ {
			sent := time.Now()
			msg := nats.NewMsg(subject)
			msg.Data = []byte(strconv.Itoa(sequence))
			msg.Header.Set(sendTime, sent.Format(time.RFC3339Nano))
			check(conn.PublishMsg(msg))
			fmt.Printf("SEND payload=%d sent=%s\n", sequence, sent.Format(time.RFC3339Nano))
			time.Sleep(time.Second)
		}
		check(conn.FlushTimeout(messageWait))
		close(published)
		<-shutdown
	})
	<-subscribed
	<-publisherReady
	config.Latency = linkLatency
	check(simnet.Configure(config))
	close(begin)
	<-published
	<-received
	close(shutdown)
}
