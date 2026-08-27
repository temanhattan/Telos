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
//   - Denies dangerous syscalls outright (ptrace, mount, reboot, etc.)
//   - Denies setns and unshare outright (no namespace manipulation)
//   - Allows clone/clone3 for normal threading but denies CLONE_NEW* flags
//   - Returns EPERM (not SIGKILL) for graceful error handling
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
	prog = append(prog, bpfInsn{Code: bpfLD | bpfW | bpfABS, K: uint32(offsetArch)})

	// Check arch == AUDIT_ARCH_X86_64, deny if mismatch.
	prog = append(prog, bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: 0xc000003e, Jt: 1, Jf: 0})
	prog = append(prog, bpfInsn{Code: bpfRET | bpfK, K: errnoEPERM})

	// Load syscall number.
	prog = append(prog, bpfInsn{Code: bpfLD | bpfW | bpfABS, K: uint32(offsetNR)})

	// Deny each listed syscall.
	for _, nr := range deniedSyscalls {
		prog = append(prog, bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: nr, Jt: 0, Jf: 1})
	}

	// Check clone.
	cloneCheckStart := len(prog)
	prog = append(prog, bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: unix.SYS_CLONE, Jt: 0, Jf: 1})

	// Check clone3.
	clone3CheckIdx := len(prog)
	prog = append(prog, bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: 435, Jt: 0, Jf: 1})

	// Default allow.
	allowIdx := len(prog)
	prog = append(prog, bpfInsn{Code: bpfRET | bpfK, K: seccompRetAllow})

	// Clone flag check: load args[0], AND with CLONE_NEW* mask.
	flagCheckStart := len(prog)
	prog = append(prog, bpfInsn{Code: bpfLD | bpfW | bpfABS, K: uint32(offsetArgs)})
	prog = append(prog, bpfInsn{Code: bpfALU | bpfAND | bpfK, K: cloneNewAllMask})
	prog = append(prog, bpfInsn{Code: bpfJMP | bpfJEQ | bpfK, K: 0, Jt: 1, Jf: 0})

	// Deny (namespace flags set).
	denyIdx := len(prog)
	prog = append(prog, bpfInsn{Code: bpfRET | bpfK, K: errnoEPERM})

	// Allow (normal clone).
	prog = append(prog, bpfInsn{Code: bpfRET | bpfK, K: seccompRetAllow})

	// Fix jump targets for denied syscalls.
	firstDenyIdx := 4
	for i := firstDenyIdx; i < cloneCheckStart; i++ {
		if prog[i].Code == bpfJMP|bpfJEQ|bpfK {
			prog[i].Jt = uint8(denyIdx - i - 1)
		}
	}

	// clone/clone3 -> flag check.
	prog[cloneCheckStart].Jt = uint8(flagCheckStart - cloneCheckStart - 1)
	prog[clone3CheckIdx].Jt = uint8(flagCheckStart - clone3CheckIdx - 1)
	prog[clone3CheckIdx].Jf = uint8(allowIdx - clone3CheckIdx - 1)

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

	fprog := sockFprog{
		Len:    uint16(len(filter)),
		Filter: &filter[0],
	}

	_, _, errno := unix.RawSyscall(
		unix.SYS_SECCOMP,
		1, // SECCOMP_SET_MODE_FILTER
		0,
		uintptr(unsafe.Pointer(&fprog)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}
