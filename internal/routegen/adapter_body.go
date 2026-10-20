package routegen

import (
	"fmt"
	"strings"

	"github.com/neria-cloud/gosim/internal/compat/spec"
)

func adapterBody(h spec.Hook, m Manifest) (string, error) {
	op := adapterOperations[h.Symbol]
	call, require, reject := "Call", "Require", "Reject"
	if h.Owner != "kernel" {
		call, require, reject = "kh.Call", "kh.Require", "kh.Reject"
	}
	prefix := ""
	if h.Owner == "fs" {
		prefix = require + "(contracts.FS," + fmt.Sprintf("%q", h.Package+"."+h.Symbol) + ");\n"
	}
	if h.Owner == "net" {
		prefix = require + "(contracts.Net," + fmt.Sprintf("%q", h.Package+"."+h.Symbol) + ");\n"
	}
	body := ""
	switch op {
	case "Reject":
		// All raw entry points reject, including known syscall numbers: named typed
		// wrappers are the only supported route into modeled syscalls.
		body = reject + "(" + fmt.Sprintf("%q", h.Package+"."+h.Symbol) + "); panic(\"unreachable raw syscall\")"
	case "Getrlimit":
		body = "if p0!=7 {Reject(\"getrlimit resource\")}; return syscall.ENOSYS"
	case "FcntlInt":
		body = "value,err:=fcntlInt(int(p0),int(p1),int(p2)); return value,err"
		if h.Package == "internal/syscall/unix" {
			body = "value,err:=fcntlInt(int(p0),int(p1),int(p2));if err!=nil {errno,ok:=err.(syscall.Errno);if !ok{Reject(\"fcntl error\")};return -1,int32(errno)};return int32(value),0"
		}
	case "FcntlLock":
		body = "return fcntlLock(int(p0),p1,unsafe.Pointer(p2))"
	case "Fstat":
		body = "if p1==nil{return syscall.EFAULT}; out,err:=" + call + "(k.FstatRequest{FD:p0},k.DecodeFstatResult);if err==nil{writeStat(unsafe.Pointer(p1),out.Stat)};return err"
	case "FstatFS":
		body = "if p1==nil{return syscall.EFAULT}; out,err:=" + call + "(k.FstatFSRequest{FD:p0},k.DecodeFstatFSResult);if err==nil{writeStatFS(unsafe.Pointer(p1),out.Stats)};return err"
	case "StatFS":
		body = "if strings.IndexByte(p0,0)>=0{return syscall.EINVAL};if p1==nil{return syscall.EFAULT};out,err:=" + call + "(k.StatFSRequest{Path:p0},k.DecodeStatFSResult);if err==nil{writeStatFS(unsafe.Pointer(p1),out.Stats)};return err"
	case "Statat":
		body = "if strings.IndexByte(p1,0)>=0{return syscall.EINVAL};if p2==nil{return syscall.EFAULT};out,err:=" + call + "(k.StatatRequest{DirFD:p0,Path:p1,Flags:p3},k.DecodeStatatResult);if err==nil{writeStat(unsafe.Pointer(p2),out.Stat)};return err"
	case "Uname":
		body = "if p0==nil{return syscall.EFAULT};out,err:=Call(k.UnameRequest{},k.DecodeUnameResult);if err==nil{writeUname(unsafe.Pointer(p0),out.Info)};return err"
	case "Bind", "Connect":
		body = "address,err:=readAddress(p1,p2);if err!=nil{return err};_,err=" + call + "(k." + op + "Request{FD:p0,Address:address},k.Decode" + op + "Result);return err"
	case "Accept":
		body = "if err:=checkAddressOutput(unsafe.Pointer(p1),p2);err!=nil{return 0,err};out,err:=" + call + "(k.AcceptRequest{FD:p0,Flags:p3},k.DecodeAcceptResult);if err==nil{writeAddress(unsafe.Pointer(p1),p2,out.Address)};return out.FD,err"
	case "LocalAddress", "PeerAddress":
		body = "if err:=checkAddressOutput(unsafe.Pointer(p1),p2);err!=nil{return err};out,err:=" + call + "(k." + op + "Request{FD:p0},k.Decode" + op + "Result);if err==nil{writeAddress(unsafe.Pointer(p1),p2,out.Address)};return err"
	case "SetOption":
		body = "if p4>uintptr(^uint32(0)){return syscall.EINVAL};if p3==nil && p4!=0{return syscall.EFAULT};simrt.RaceReadRange(p3,int(p4));_,err:=" + call + "(k.SetOptionRequest{FD:p0,Level:p1,Name:p2,Value:abi.NewSliceView((*byte)(p3),p4)},k.DecodeSetOptionResult);return err"
	case "GetOption":
		body = "if p4==nil{return syscall.EFAULT};simrt.RaceReadRange(unsafe.Pointer(p4),4);size:=abi.NewValueView(p4).Get();if p3==nil && size!=0{return syscall.EFAULT};out,err:=" + call + "(k.GetOptionRequest{FD:p0,Level:p1,Name:p2,Into:abi.NewSliceView((*byte)(p3),uintptr(size))},k.DecodeGetOptionResult);if err==nil{simrt.RaceWriteRange(p3,out.Count);simrt.RaceWriteRange(unsafe.Pointer(p4),4);abi.NewValueView(p4).Set(uint32(out.Count))};return err"
	default:
		routeName := op
		if op == "DataSync" {
			routeName = "Sync"
		}
		if op == "VGetrandom" {
			routeName = "Getrandom"
		}
		var r *Route
		for _, item := range m.Routes {
			if item.Name == routeName {
				routeCopy := item
				r = &routeCopy
				break
			}
		}
		if r == nil {
			return "", fmt.Errorf("missing adapter route %s", op)
		}
		params := make([]string, len(r.Inputs))
		var before, after string
		for i, f := range r.Inputs {
			arg := fmt.Sprintf("p%d", i)
			if f.Type == "abi.ByteView" {
				if op == "Write" || op == "Pwrite" || op == "Advise" {
					before += "simrt.RaceReadRange(unsafe.Pointer(unsafe.SliceData(" + arg + ")),len(" + arg + "));\n"
				} else {
					after += "if out.Count>0{simrt.RaceWriteRange(unsafe.Pointer(unsafe.SliceData(" + arg + ")),out.Count)};\n"
				}
				arg = "abi.ByteView{Ptr:" + arg + "}"
			}
			if f.Type == "string" {
				returns := "return syscall.EINVAL"
				if len(r.Results) > 0 {
					returns = "return 0,syscall.EINVAL"
				}
				before += "if strings.IndexByte(" + arg + ",0)>=0{" + returns + "};\n"
			}
			if routeName == "Sync" && i == 1 {
				arg = fmt.Sprint(op == "DataSync")
			}
			if routeName == "Getrandom" && i == 1 {
				arg = "int(p1)"
			}
			params[i] = f.Name + ":" + arg
		}
		variable := "_"
		if len(r.Results) > 0 {
			variable = "out"
		}
		body = before + variable + ",err:=" + call + "(k." + routeName + "Request{" + strings.Join(params, ",") + "},k.Decode" + routeName + "Result);\n" + after
		if op == "Getpid" {
			body += "if err!=nil{Reject(\"getpid\")};return out.PID"
		} else if op == "VGetrandom" {
			body += "if err!=nil{errno,ok:=err.(syscall.Errno);if !ok{Reject(\"getrandom error\")};return -int(errno),true};return out.Count,true"
		} else if len(r.Results) == 0 {
			body += "return err"
		} else {
			body += "return out." + r.Results[0].Name + ",err"
		}
	}
	return prefix + body, nil
}

func adapterHelpers(owner string) string {
	switch owner {
	case "kernel":
		return `
func fcntlInt(fd,command,argument int)(int,error){
 if command==5 || command==6 || command==7 {Require(contracts.FS,"fcntl pointer command");return 0,syscall.EINVAL}
 out,err:=Call(k.FcntlIntRequest{FD:fd,Command:command,Argument:argument},k.DecodeFcntlIntResult);return out.Value,err
}
func writeUname(p unsafe.Pointer,info k.ProcessInfo){
 var out Utsname
 put:=func(dst []int8,value string){for i:=0;i<len(value) && i<len(dst)-1;i++{dst[i]=int8(value[i])}}
 put(out.Sysname[:],info.Sysname);put(out.Nodename[:],info.Nodename);put(out.Release[:],info.Release);put(out.Version[:],info.Version);put(out.Machine[:],info.Machine)
 simrt.RaceWriteRange(p,int(unsafe.Sizeof(out)));abi.NewValueView((*Utsname)(p)).Set(out)
}
`
	case "fs":
		return `
func writeStat(p unsafe.Pointer,stat contracts.Stat){
 ns:=func(n int64)kh.Timespec {sec,rem:=n/1e9,n%1e9;if rem<0{sec--;rem+=1e9};return kh.Timespec{Sec:sec,Nsec:rem}}
 out:=kh.Stat_t{Ino:stat.Inode,Size:stat.Size,Mode:stat.Mode,Uid:stat.UID,Gid:stat.GID,Nlink:stat.Nlink,Atim:ns(stat.AtimeNS),Mtim:ns(stat.MtimeNS),Ctim:ns(stat.CtimeNS)}
 simrt.RaceWriteRange(p,int(unsafe.Sizeof(out)));abi.NewValueView((*kh.Stat_t)(p)).Set(out)
}
func writeStatFS(p unsafe.Pointer,stats contracts.FSStorageStats){
 flags:=int64(0);if stats.ReadOnly{flags=1}
 out:=kh.Statfs_t{Type:0x01021994,Bsize:int64(stats.BlockSize),Blocks:stats.TotalBytes/stats.BlockSize,Bfree:stats.FreeBytes/stats.BlockSize,Bavail:stats.FreeBytes/stats.BlockSize,Frsize:int64(stats.BlockSize),Namelen:255,Flags:flags}
 simrt.RaceWriteRange(p,int(unsafe.Sizeof(out)));abi.NewValueView((*kh.Statfs_t)(p)).Set(out)
}
func fcntlLock(fd,command int,p unsafe.Pointer)error{
 if command!=5 && command!=6 && command!=7{return syscall.EINVAL}
 if p==nil{return syscall.EFAULT}
 simrt.RaceReadRange(p,int(unsafe.Sizeof(kh.Flock_t{})))
 input:=abi.NewValueView((*kh.Flock_t)(p)).Get()
 lock:=contracts.Lock{Type:input.Type,Whence:input.Whence,Start:input.Start,Len:input.Len,PID:input.Pid}
 _,err:=kh.Call(k.FcntlLockRequest{FD:fd,Command:command,Lock:abi.NewValueView(&lock)},k.DecodeFcntlLockResult)
 if err==nil && command==5 {input.Type=lock.Type;input.Whence=lock.Whence;input.Start=lock.Start;input.Len=lock.Len;input.Pid=lock.PID;simrt.RaceWriteRange(p,int(unsafe.Sizeof(input)));abi.NewValueView((*kh.Flock_t)(p)).Set(input)}
 return err
}
`
	case "net":
		return `
func readAddress(p unsafe.Pointer,length uint32)(contracts.Address,error){
 if p==nil{return contracts.Address{},syscall.EFAULT};if length<2{return contracts.Address{},syscall.EINVAL}
 simrt.RaceReadRange(p,2);family:=abi.NewValueView((*uint16)(p)).Get()
 if family!=2{return contracts.Address{},syscall.EAFNOSUPPORT};if length<16{return contracts.Address{},syscall.EINVAL}
 simrt.RaceReadRange(p,16);raw:=abi.NewValueView((*kh.RawSockaddrInet4)(p)).Get()
 return contracts.Address{Family:raw.Family,IPv4:raw.Addr,Port:raw.Port>>8|raw.Port<<8},nil
}
func checkAddressOutput(p unsafe.Pointer,length *uint32)error{
 if p==nil || length==nil{return syscall.EFAULT};simrt.RaceReadRange(unsafe.Pointer(length),4)
 if abi.NewValueView(length).Get()<16{return syscall.EINVAL};return nil
}
func writeAddress(p unsafe.Pointer,length *uint32,address contracts.Address){
 raw:=kh.RawSockaddrInet4{Family:address.Family,Port:address.Port>>8|address.Port<<8,Addr:address.IPv4}
 simrt.RaceWriteRange(p,16);abi.NewValueView((*kh.RawSockaddrInet4)(p)).Set(raw)
 simrt.RaceWriteRange(unsafe.Pointer(length),4);abi.NewValueView(length).Set(16)
}
`
	}
	panic(owner)
}
