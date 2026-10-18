//go:build sim

package etcd_test

import (
	"context"
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
	"github.com/neria-cloud/gosim/nemesis"
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

// runEtcdNode runs an etcd node with the configuration from
// https://etcd.io/docs/v3.5/op-guide/clustering/
func runEtcdNode(name, addr string) {
	cfg := embed.NewConfig()
	cfg.Name = name

	cfg.QuotaBackendBytes = 1024 * 1024 // passed to mmap

	cfg.ListenPeerUrls = parseUrls(fmt.Sprintf("http://%s:2380", addr))
	cfg.AdvertisePeerUrls = parseUrls(fmt.Sprintf("http://%s:2380", addr))

	cfg.ListenClientUrls = parseUrls(fmt.Sprintf("http://%s:2379", addr))
	cfg.AdvertiseClientUrls = parseUrls(fmt.Sprintf("http://%s:2379", addr))

	cfg.InitialCluster = "etcd-1=http://10.0.0.1:2380,etcd-2=http://10.0.0.2:2380,etcd-3=http://10.0.0.3:2380"
	cfg.ClusterState = embed.ClusterStateFlagNew
	cfg.InitialClusterToken = "toktok"

	cfg.ZapLoggerBuilder = embed.NewZapLoggerBuilder(etcd.MakeZapLogger())

	etcd.EtcdMain(cfg)
}

// TestEtcd runs a 3 node etcd cluster, partitions the network between the
// nodes, and makes sure key-value puts and gets work.
func TestEtcd(t *testing.T) {
	gosim.SetSimulationTimeout(2 * time.Minute)

	// run machines:
	gosim.NewMachine(gosim.MachineConfig{
		Label: "etcd-1",
		Addr:  netip.MustParseAddr("10.0.0.1"),
		MainFunc: func() {
			runEtcdNode("etcd-1", "10.0.0.1")
		},
	})
	gosim.NewMachine(gosim.MachineConfig{
		Label: "etcd-2",
		Addr:  netip.MustParseAddr("10.0.0.2"),
		MainFunc: func() {
			time.Sleep(100 * time.Millisecond)
			runEtcdNode("etcd-2", "10.0.0.2")
		},
	})
	gosim.NewMachine(gosim.MachineConfig{
		Label: "etcd-3",
		Addr:  netip.MustParseAddr("10.0.0.3"),
		MainFunc: func() {
			time.Sleep(200 * time.Millisecond)
			runEtcdNode("etcd-3", "10.0.0.3")
		},
	})

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
	defer client.Close()

	// use the client
	attempts := 0
	successes := 0
	doPutGet := func() {
		attempts++

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		key := fmt.Sprintf("ctr%d", attempts)
		val := fmt.Sprintf("val%d", attempts)

		log.Printf("calling KV.Put %s=%s", key, val)
		resp, err := client.KV.Put(ctx, key, val)
		if err != nil {
			log.Printf("KV.Put failed: %s", err)
			return
		}
		log.Printf("KV.Put succeeded: revision %d", resp.Header.Revision)

		log.Printf("calling KV.Get %s", key)
		resp2, err := client.KV.Get(ctx, key)
		if err != nil {
			log.Printf("KV.Get failed: %s", err)
			return
		}

		got := string(resp2.Kvs[0].Value)
		log.Printf("KV.Get %s: %s", key, got)
		if got != val {
			log.Printf("unexpected KV value: %s", got)
			return
		}

		successes++
	}

	for i := 0; i < 60; i++ {
		doPutGet()
		time.Sleep(time.Second)
	}

	log.Printf("%d successes out of %d attempts", successes, attempts)

	if successes == 0 {
		t.Errorf("got no succeses")
	}

	// TODO investigate:
	// - this listener port doesn't make sense when passing localhost:2379.
	//   where does it come from? (could try debugging)
	// - what happens when you listen on a bad ip?
}
