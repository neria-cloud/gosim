//go:build go1.25 && race

package runtimebridge

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"unsafe"
)

// Match upstream's single pointer slot, independently of the target's Go type.
//
//go:linkname objectRead github.com/neria-cloud/gosim/internal/compat/shims/runtimebridge.InternalRace_ReadObjectPC
func objectRead(*struct{}, unsafe.Pointer, uintptr, uintptr)

//go:linkname objectWrite github.com/neria-cloud/gosim/internal/compat/shims/runtimebridge.InternalRace_WriteObjectPC
func objectWrite(*struct{}, unsafe.Pointer, uintptr, uintptr)

func TestObjectRaceABI(t *testing.T) {
	if mode := os.Getenv("GOSIM_OBJECT_RACE_CHILD"); mode != "" {
		data := new([8]uintptr)
		value := any(*data)
		// internal/abi.EmptyInterface.Type is the native immutable type descriptor.
		typ := (*struct {
			typ  unsafe.Pointer
			data unsafe.Pointer
		})(unsafe.Pointer(&value)).typ
		pc, _, _, _ := runtime.Caller(0)
		entry := runtime.FuncForPC(pc).Entry()
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			for range 20 {
				if mode == "read" {
					objectRead((*struct{})(typ), unsafe.Pointer(data), pc, entry)
				} else {
					objectWrite((*struct{})(typ), unsafe.Pointer(data), pc, entry)
				}
				runtime.KeepAlive(value)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			var sum uintptr
			for range 20 {
				if mode == "read" {
					data[7]++
				} else {
					sum += data[7]
				}
			}
			runtime.KeepAlive(sum)
		}()
		close(start)
		wg.Wait()
		return
	}
	for _, mode := range []string{"read", "write"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestObjectRaceABI$")
			cmd.Env = append(os.Environ(), "GOSIM_OBJECT_RACE_CHILD="+mode, "GORACE=halt_on_error=1")
			out, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(out, []byte("WARNING: DATA RACE")) || !bytes.Contains(out, []byte("TestObjectRaceABI")) {
				t.Fatalf("missing last-field object race: %v\n%s", err, out)
			}
		})
	}
}
