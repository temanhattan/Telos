//go:build linux

package sandbox

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// seccomp BPF constants.
const (
	bpfLD  = 0x00
	bpfW   = 0x00
	bpfABS = 0x20
	bpfJMP = 0x05
	bpfJEQ = 0x10
	bpfRET = 0x06
	bpfK   = 0x00
	bpfAND = 0x50
	bpfALU = 0x04

	seccompRetAllow = 0x7fff0000
	errnoEPERM      = 0x00050001
	errnoENOSYS     = 0x00050026

	offsetNR   = 0
	offsetArch = 4
	offsetArgs = 16

	cloneNewUser    = 0x10000000
	cloneNewPID     = 0x20000000
	cloneNewNet     = 0x40000000
	cloneNewMount   = 0x00020000
	cloneNewUTS     = 0x04000000
	cloneNewIPC     = 0x08000000
	cloneNewCgroup  = 0x02000000
	cloneNewTime    = 0x00000080
	cloneNewAllMask = cloneNewUser | cloneNewPID | cloneNewNet | cloneNewMount |
		cloneNewUTS | cloneNewIPC | cloneNewCgroup | cloneNewTime
)

type bpfInsn struct {
	Code uint16
	Jt   uint8
	Jf   uint8
	K    uint32
}

// buildSeccompFilter constructs the defense-in-depth syscall denylist.
//
// This is NOT a complete syscall confinement boundary. It reduces the attack
// surface by denying obviously dangerous operations. A plugin retains access
// to all syscalls not explicitly denied.
//
// Key behaviors:
//   - Denies dangerous syscalls outright (ptrace, mount, reboot, etc.) with EPERM
//   - Denies setns and unshare outright (no namespace manipulation) with EPERM
//   - Allows clone for normal threading but denies CLONE_NEW* flags with EPERM
//   - Unconditionally denies clone3 with ENOSYS to prevent namespace creation while triggering libc fallback
//   - Validates AUDIT_ARCH_X86_64 to prevent 32-bit compat bypass
func buildSeccompFilter() []bpfInsn {
	deniedSyscalls := []uint32{
		unix.SYS_PTRACE,
		unix.SYS_MOUNT,
		unix.SYS_UMOUNT2,
		unix.SYS_REBOOT,
		unix.SYS_KEXEC_LOAD,
		unix.SYS_INIT_MODULE,
		unix.SYS_FINIT_MODULE,
		unix.SYS_DELETE_MODULE,
		unix.SYS_PIVOT_ROOT,
		unix.SYS_CHROOT,
		unix.SYS_SWAPON,
		unix.SYS_SWAPOFF,
		unix.SYS_SETNS,
		unix.SYS_UNSHARE,
		unix.SYS_ACCT,
		unix.SYS_SETTIMEOFDAY,
		unix.SYS_CLOCK_SETTIME,
		unix.SYS_ADJTIMEX,
		320, // SYS_KEXEC_FILE_LOAD
	}

	var prog []bpfInsn

	// Load arch.
	// Check arch == AUDIT_ARCH_X86_64, deny if mismatch.
	// Load syscall number.
	prog = append(prog,
		bpfInsn{Code: bpfLD | bpfW | bpfABS, K: uint32(offsetArch)},
		bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: 0xc000003e, Jt: 1, Jf: 0},
		bpfInsn{Code: bpfRET | bpfK, K: errnoEPERM},
		bpfInsn{Code: bpfLD | bpfW | bpfABS, K: uint32(offsetNR)},
	)

	// Deny each listed syscall with EPERM.
	for _, nr := range deniedSyscalls {
		// Jf: 0 ensures we fall through to the next check if the syscall doesn't match.
		prog = append(prog, bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: nr, Jt: 0, Jf: 0})
	}

	// Check clone3 (syscall 435). We deny it unconditionally with ENOSYS.
	clone3CheckIdx := len(prog)
	prog = append(prog, bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: 435, Jt: 0, Jf: 0})

	// Check clone and Default allow.
	cloneCheckStart := len(prog)
	prog = append(prog,
		bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: unix.SYS_CLONE, Jt: 0, Jf: 0},
		bpfInsn{Code: bpfRET | bpfK, K: seccompRetAllow},
	)

	// Clone flag check: load args[0], AND with CLONE_NEW* mask.
	flagCheckStart := len(prog)
	prog = append(prog,
		bpfInsn{Code: bpfLD | bpfW | bpfABS, K: uint32(offsetArgs)},
		bpfInsn{Code: bpfALU | bpfAND | bpfK, K: cloneNewAllMask},
		bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: 0, Jt: 1, Jf: 0},
	)

	// EPERM return (for denied syscalls and clone flags).
	denyEPERMIdx := len(prog)
	prog = append(prog,
		bpfInsn{Code: bpfRET | bpfK, K: errnoEPERM},
		bpfInsn{Code: bpfRET | bpfK, K: seccompRetAllow},
	)

	// ENOSYS return (for clone3).
	denyENOSYSIdx := len(prog)
	prog = append(prog, bpfInsn{Code: bpfRET | bpfK, K: errnoENOSYS})

	// Fix jump targets.
	// 1. All deniedSyscalls jump to denyEPERMIdx if true.
	firstDenyIdx := 4
	for i := firstDenyIdx; i < clone3CheckIdx; i++ {
		jt := denyEPERMIdx - i - 1
		if jt < 0 || jt > 255 {
			panic("seccomp jump target out of bounds")
		}
		prog[i].Jt = uint8(jt)
	}

	// 2. clone3 jumps to denyENOSYSIdx if true.
	jtClone3 := denyENOSYSIdx - clone3CheckIdx - 1
	if jtClone3 < 0 || jtClone3 > 255 {
		panic("seccomp jump target out of bounds")
	}
	prog[clone3CheckIdx].Jt = uint8(jtClone3)

	// 3. clone jumps to flagCheckStart if true.
	jtClone := flagCheckStart - cloneCheckStart - 1
	if jtClone < 0 || jtClone > 255 {
		panic("seccomp jump target out of bounds")
	}
	prog[cloneCheckStart].Jt = uint8(jtClone)

	return prog
}

// installSeccomp applies the BPF filter via the seccomp syscall.
// Must be called after prctl(PR_SET_NO_NEW_PRIVS, 1).
func installSeccomp() error {
	filter := buildSeccompFilter()

	type sockFprog struct {
		Len    uint16
		_      [6]byte
		Filter *bpfInsn
	}

	if len(filter) > 65535 {
		return unix.E2BIG
	}

	fprog := sockFprog{
		Len:    uint16(len(filter)), // #nosec G115 -- length is bounds-checked above
		Filter: &filter[0],
	}

	_, _, errno := unix.RawSyscall(
		unix.SYS_SECCOMP,
		1, // SECCOMP_SET_MODE_FILTER
		0,
		uintptr(unsafe.Pointer(&fprog)), // #nosec G103 -- required for seccomp syscall
	)
	if errno != 0 {
		return errno
	}
	return nil
}
