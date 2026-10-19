//go:build gosim_retired

package go123

func InternalCpu_getisar0() uint64 {
	panic("gosim not implemented")
}

func InternalCpu_getMIDR() uint64 {
	panic("gosim not implemented")
}

func InternalCpu_getpfr0() uint64 {
	panic("gosim not implemented")
}

func InternalCpu_cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32) {
	panic("gosim not implemented")
}

func InternalCpu_xgetbv() (eax, edx uint32) {
	panic("gosim not implemented")
}

func InternalCpu_getGOAMD64level() int32 {
	panic("gosim not implemented")
}
