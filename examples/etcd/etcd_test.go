//go:build sim

package etcd_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"net/url"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"

	"github.com/neria-cloud/gosim"
	"github.com/neria-cloud/gosim/examples/etcd"
	simfs "github.com/neria-cloud/gosim/fs"
	"github.com/neria-cloud/gosim/nemesis"
	simnet "github.com/neria-cloud/gosim/net"
	"github.com/neria-cloud/gosim/rt"
)

func parseUrls(a ...string) []url.URL {
	var us []url.URL
	for _, s := range a {
		u, err := url.Parse(s)
		if err != nil {
			panic(err)
		}
		us = append(us, *u)
	}
	return us
}

type nodeHandle struct {
	Machine gosim.Machine
	Cancel  context.CancelFunc
	Ready   <-chan error
	Done    <-chan error
}

type phaseResult struct{ Attempts, Successes int }

// nodeConfig follows etcd's static-cluster configuration, with each node's
// data directory and listeners owned by its simulated machine.
func nodeConfig(name, addr string) *embed.Config {
	cfg := embed.NewConfig()
	cfg.Name = name
	cfg.Dir = name + "-data"

	cfg.QuotaBackendBytes = 1024 * 1024 // passed to mmap

	cfg.ListenPeerUrls = parseUrls(fmt.Sprintf("http://%s:2380", addr))
	cfg.AdvertisePeerUrls = parseUrls(fmt.Sprintf("http://%s:2380", addr))

	cfg.ListenClientUrls = parseUrls(fmt.Sprintf("http://%s:2379", addr))
	cfg.AdvertiseClientUrls = parseUrls(fmt.Sprintf("http://%s:2379", addr))

	cfg.InitialCluster = "etcd-1=http://10.0.0.1:2380,etcd-2=http://10.0.0.2:2380,etcd-3=http://10.0.0.3:2380"
	cfg.ClusterState = embed.ClusterStateFlagNew
	cfg.InitialClusterToken = "toktok"
	cfg.ClientAutoTLS = false
	cfg.PeerAutoTLS = false
	cfg.ExperimentalEnableDistributedTracing = false
	cfg.ListenMetricsUrls = nil
	cfg.EnableV2 = false

	cfg.ZapLoggerBuilder = embed.NewZapLoggerBuilder(etcd.MakeZapLogger())
	return cfg
}

func startNode(t *testing.T, name, address string) nodeHandle {
	t.Helper()
	ready := make(chan error, 1)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cfg := nodeConfig(name, address)
	m, err := gosim.NewMachine(gosim.MachineConfig{Label: name, Main: func() {
		done <- etcd.RunEmbedded(ctx, cfg, ready)
	}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		state, err := m.State()
		if err == nil && (state == gosim.Starting || state == gosim.Running) {
			_ = m.Stop()
		}
	})
	if err := simnet.ConfigureMachine(m, simnet.MachineConfig{Addr: netip.MustParseAddr(address)}); err != nil {
		cancel()
		t.Fatal(err)
	}
	fsConfig := simfs.DefaultConfig()
	fsConfig.CapacityBytes = 128 << 20
	if err := simfs.ConfigureMachine(m, fsConfig); err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	return nodeHandle{Machine: m, Cancel: cancel, Ready: ready, Done: done}
}

func waitReady(t *testing.T, node nodeHandle) {
	t.Helper()
	select {
	case err := <-node.Ready:
		if err != nil {
			t.Fatalf("etcd startup failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("etcd readiness deadline exceeded")
	}
}

func stopNode(t *testing.T, node nodeHandle) {
	t.Helper()
	node.Cancel()
	select {
	case err := <-node.Done:
		if err != nil {
			t.Errorf("etcd shutdown failed: %v", err)
		}
	case <-time.After(20 * time.Second):
		_ = node.Machine.Stop()
		t.Fatal("etcd shutdown deadline exceeded")
	}
	exit, err := node.Machine.Wait()
	if err != nil || exit.Kind != gosim.Graceful {
		t.Errorf("etcd machine exit: %+v, %v", exit, err)
	}
}

func exerciseKV(t *testing.T, client *clientv3.Client, phase string, duration time.Duration) phaseResult {
	t.Helper()
	result := phaseResult{}
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		result.Attempts++
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		key := fmt.Sprintf("%s-%d", phase, result.Attempts)
		value := fmt.Sprintf("value-%s-%d", phase, result.Attempts)
		_, err := client.KV.Put(ctx, key, value)
		if err == nil {
			response, getErr := client.KV.Get(ctx, key)
			if getErr == nil {
				if len(response.Kvs) != 1 || string(response.Kvs[0].Value) != value {
					t.Errorf("%s: key %q returned unexpected value: %+v", phase, key, response.Kvs)
				} else {
					result.Successes++
				}
			}
		}
		cancel()
		time.Sleep(time.Second)
	}
	return result
}

func TestRunEmbeddedInvalidConfig(t *testing.T) {
	invalidCluster := nodeConfig("etcd-1", "10.0.0.1")
	invalidCluster.TickMs = 0
	for _, cfg := range []*embed.Config{nil, invalidCluster} {
		ready := make(chan error, 1)
		err := etcd.RunEmbedded(context.Background(), cfg, ready)
		if err == nil {
			t.Fatal("invalid embedded config succeeded")
		}
		select {
		case startupErr := <-ready:
			if startupErr == nil || startupErr.Error() != err.Error() {
				t.Fatalf("startup error=%v, return=%v", startupErr, err)
			}
		default:
			t.Fatal("startup error was not published")
		}
	}
}

func TestRunEmbeddedCancelledBeforeReady(t *testing.T) {
	if err := rt.SetTimeout(30 * time.Second); err != nil {
		t.Fatal(err)
	}
	ready := make(chan error, 1)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m, err := gosim.NewMachine(gosim.MachineConfig{Label: "cancel-before-ready", Main: func() {
		done <- etcd.RunEmbedded(ctx, nodeConfig("etcd-1", "10.0.0.1"), ready)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := simnet.ConfigureMachine(m, simnet.MachineConfig{Addr: netip.MustParseAddr("10.0.0.1")}); err != nil {
		t.Fatal(err)
	}
	if err := simfs.ConfigureMachine(m, simfs.DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-ready:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("startup after cancellation: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("canceled startup did not report")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled startup return: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("canceled server did not stop")
	}
	if exit, err := m.Wait(); err != nil || exit.Kind != gosim.Graceful {
		t.Fatalf("canceled machine exit: %+v, %v", exit, err)
	}
}

// TestEtcd runs a 3 node etcd cluster, partitions the network between the
// nodes, and makes sure key-value puts and gets work.
func TestEtcd(t *testing.T) {
	if err := rt.SetTimeout(2 * time.Minute); err != nil {
		t.Fatal(err)
	}
	var nodes []nodeHandle
	for i := 1; i <= 3; i++ {
		label, addr := fmt.Sprintf("etcd-%d", i), fmt.Sprintf("10.0.0.%d", i)
		nodes = append(nodes, startNode(t, label, addr))
	}
	for _, node := range nodes {
		waitReady(t, node)
	}

	// mess with the network in the background
	go nemesis.Sequence(
		nemesis.Sleep{
			Duration: 10 * time.Second,
		},
		nemesis.PartitionMachines{
			Addresses: []string{
				"10.0.0.1",
				"10.0.0.2",
				"10.0.0.3",
			},
			Duration: 30 * time.Second,
		},
	).Run()

	// build a client
	client, err := clientv3.New(clientv3.Config{
		Endpoints:        []string{"10.0.0.1:2379", "10.0.0.2:2379", "10.0.0.3:2379"},
		DialTimeout:      5 * time.Second,
		AutoSyncInterval: 5 * time.Second,
		Logger:           etcd.MakeZapLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	result := exerciseKV(t, client, "initial", time.Minute)
	log.Printf("%d successes out of %d attempts", result.Successes, result.Attempts)
	if result.Successes == 0 {
		t.Errorf("got no succeses")
	}
	if err := client.Close(); err != nil {
		t.Error(err)
	}
	for _, node := range nodes {
		stopNode(t, node)
	}

	// TODO investigate:
	// - this listener port doesn't make sense when passing localhost:2379.
	//   where does it come from? (could try debugging)
	// - what happens when you listen on a bad ip?
}
