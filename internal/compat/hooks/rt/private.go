//go:build go1.25

package rt

import (
	"unsafe"

	simrt "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func rejectPrivate(operation string) {
	simrt.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: operation, Message: "native CPU and function-ABI operations are not modeled"})
}
func InternalAbi_FuncPCABI0(f any) uintptr { rejectPrivate("internal/abi.FuncPCABI0"); return 0 }
func InternalAbi_FuncPCABIInternal(f any) uintptr {
	rejectPrivate("internal/abi.FuncPCABIInternal")
	return 0
}

func InternalBytealg_abigen_runtime_memequal(a, b unsafe.Pointer, size uintptr) bool {
	rejectPrivate("internal/bytealg.abigen_runtime_memequal")
	return false
}

func InternalBytealg_abigen_runtime_memequal_varlen(a, b unsafe.Pointer) bool {
	rejectPrivate("internal/bytealg.abigen_runtime_memequal_varlen")
	return false
}

// Only the appropriate source declarations enter each architecture's manifest.
// Translated CPU feature variables remain zero; native runtime CPU initialization
// is not an application operation and cannot call these stubs successfully.
func InternalCpu_getisar0() uint64 { rejectPrivate("internal/cpu.getisar0"); return 0 }
func InternalCpu_getMIDR() uint64  { rejectPrivate("internal/cpu.getMIDR"); return 0 }
func InternalCpu_getpfr0() uint64  { rejectPrivate("internal/cpu.getpfr0"); return 0 }
func InternalCpu_cpuid(a, b uint32) (uint32, uint32, uint32, uint32) {
	rejectPrivate("internal/cpu.cpuid")
	return 0, 0, 0, 0
}
func InternalCpu_xgetbv() (uint32, uint32) { rejectPrivate("internal/cpu.xgetbv"); return 0, 0 }
func InternalCpu_getGOAMD64level() int32   { rejectPrivate("internal/cpu.getGOAMD64level"); return 0 }
func Net_runtime_rand() uint64             { return simrt.Fastrand64() }
