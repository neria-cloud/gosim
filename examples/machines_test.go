//go:build sim

package examples_test

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/neria-cloud/gosim"
)

var count = 0

func server() {
	log.Printf("starting server")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		count++
		log.Printf("got a request from %s", r.RemoteAddr)
		fmt.Fprintf(w, "hello from the server! request: %d", count)
	})
	http.ListenAndServe("10.0.0.1:80", nil)
}

func request() {
	log.Println("making a request")
	resp, err := http.Get("http://10.0.0.1/")
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println(string(body))
}

func TestMachines(t *testing.T) {
	// run the server
	serverMachine := gosim.NewMachine(gosim.MachineConfig{
		Label:    "server",
		Addr:     netip.MustParseAddr("10.0.0.1"),
		MainFunc: server,
	})

	// let the server start
	time.Sleep(time.Second)

	// make some requests to see them work
	request()
	request()

	// restart the server
	log.Println("restarting the server")
	serverMachine.Crash()
	serverMachine.Restart()
	time.Sleep(time.Second)

	// make a new request to see a reset count
	request()

	// add some latency
	log.Println("adding latency")
	gosim.SetDelay("10.0.0.1", "11.0.0.1", time.Second)

	// make another request to see the latency
	request()
}
