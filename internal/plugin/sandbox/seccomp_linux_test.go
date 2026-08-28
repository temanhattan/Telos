//go:build linux

package sandbox

import (
	"fmt"
	"testing"

	"golang.org/x/sys/unix"
)

// TestBuildSeccompFilterStructure structurally inspects the generated BPF
// program to verify every jump target, denylist comparison, and return action.
// This does NOT invoke the kernel — it only inspects the instruction array.
func TestBuildSeccompFilterStructure(t *testing.T) {
	prog := buildSeccompFilter()

	if len(prog) == 0 {
		t.Fatal("buildSeccompFilter returned empty program")
	}

	// --- Section 1: Architecture check (instructions 0–3) ---

	// Instruction 0: LD arch
	assertInsn(t, prog, 0, "load arch", bpfLD|bpfW|bpfABS, 0, 0, uint32(offsetArch))

	// Instruction 1: JEQ AUDIT_ARCH_X86_64, Jt=1 (skip EPERM), Jf=0 (fall through to EPERM)
	assertInsn(t, prog, 1, "check arch", bpfJMP|bpfJEQ|bpfK, 1, 0, 0xc000003e)

	// Instruction 2: RET EPERM (wrong arch)
	assertInsn(t, prog, 2, "ret EPERM (bad arch)", bpfRET|bpfK, 0, 0, errnoEPERM)

	// Instruction 3: LD syscall NR
	assertInsn(t, prog, 3, "load syscall nr", bpfLD|bpfW|bpfABS, 0, 0, uint32(offsetNR))

	// --- Section 2: Denylist comparisons (instructions 4 through 4+len(denied)-1) ---

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

	firstDenyIdx := 4
	denyCount := len(deniedSyscalls)
	clone3CheckIdx := firstDenyIdx + denyCount
	cloneCheckIdx := clone3CheckIdx + 1
	defaultAllowIdx := cloneCheckIdx + 1
	flagCheckStartIdx := defaultAllowIdx + 1
	// flagCheckStartIdx+0: LD args[0]
	// flagCheckStartIdx+1: AND cloneNewAllMask
	// flagCheckStartIdx+2: JEQ 0 (flags clean → allow, flags set → deny)
	denyEPERMIdx := flagCheckStartIdx + 3
	allowCloneIdx := denyEPERMIdx + 1
	denyENOSYSIdx := allowCloneIdx + 1

	expectedLen := denyENOSYSIdx + 1
	if len(prog) != expectedLen {
		t.Fatalf("expected %d instructions, got %d", expectedLen, len(prog))
	}

	// Verify each denylist comparison.
	for i, nr := range deniedSyscalls {
		idx := firstDenyIdx + i
		insn := prog[idx]
		name := fmt.Sprintf("deny syscall %d at idx %d", nr, idx)

		// Must be JEQ with the correct syscall number.
		if insn.Code != bpfJMP|bpfJEQ|bpfK {
			t.Errorf("%s: wrong opcode %#x", name, insn.Code)
		}
		if insn.K != nr {
			t.Errorf("%s: wrong syscall K=%d, want %d", name, insn.K, nr)
		}

		// Jf MUST be 0 (fall through to next check).
		if insn.Jf != 0 {
			t.Errorf("%s: Jf=%d, want 0 (fall-through)", name, insn.Jf)
		}

		// Jt must resolve to denyEPERMIdx.
		jtTarget := idx + 1 + int(insn.Jt)
		if jtTarget != denyEPERMIdx {
			t.Errorf("%s: Jt resolves to %d, want %d (denyEPERM)", name, jtTarget, denyEPERMIdx)
		}
	}

	// --- Section 3: clone3 check ---
	{
		insn := prog[clone3CheckIdx]
		if insn.Code != bpfJMP|bpfJEQ|bpfK {
			t.Errorf("clone3 check: wrong opcode %#x", insn.Code)
		}
		if insn.K != 435 {
			t.Errorf("clone3 check: wrong syscall K=%d, want 435", insn.K)
		}
		if insn.Jf != 0 {
			t.Errorf("clone3 check: Jf=%d, want 0", insn.Jf)
		}
		jtTarget := clone3CheckIdx + 1 + int(insn.Jt)
		if jtTarget != denyENOSYSIdx {
			t.Errorf("clone3 check: Jt resolves to %d, want %d (denyENOSYS)", jtTarget, denyENOSYSIdx)
		}
	}

	// --- Section 4: clone check ---
	{
		insn := prog[cloneCheckIdx]
		if insn.Code != bpfJMP|bpfJEQ|bpfK {
			t.Errorf("clone check: wrong opcode %#x", insn.Code)
		}
		if insn.K != unix.SYS_CLONE {
			t.Errorf("clone check: wrong syscall K=%d, want %d", insn.K, unix.SYS_CLONE)
		}
		if insn.Jf != 0 {
			t.Errorf("clone check: Jf=%d, want 0", insn.Jf)
		}
		jtTarget := cloneCheckIdx + 1 + int(insn.Jt)
		if jtTarget != flagCheckStartIdx {
			t.Errorf("clone check: Jt resolves to %d, want %d (flagCheckStart)", jtTarget, flagCheckStartIdx)
		}
	}

	// --- Section 5: Default allow ---
	assertInsn(t, prog, defaultAllowIdx, "default allow", bpfRET|bpfK, 0, 0, seccompRetAllow)

	// --- Section 6: Clone flag check ---
	// Load args[0].
	assertInsn(t, prog, flagCheckStartIdx, "load clone flags", bpfLD|bpfW|bpfABS, 0, 0, uint32(offsetArgs))
	// AND with CLONE_NEW* mask.
	assertInsn(t, prog, flagCheckStartIdx+1, "AND clone mask", bpfALU|bpfAND|bpfK, 0, 0, cloneNewAllMask)
	// JEQ 0: if flags & mask == 0, Jt=1 → skip EPERM → allow. Jf=0 → fall through to EPERM.
	{
		insn := prog[flagCheckStartIdx+2]
		if insn.Code != bpfJMP|bpfJEQ|bpfK {
			t.Errorf("clone flag JEQ: wrong opcode %#x", insn.Code)
		}
		if insn.K != 0 {
			t.Errorf("clone flag JEQ: K=%d, want 0", insn.K)
		}
		if insn.Jt != 1 {
			t.Errorf("clone flag JEQ: Jt=%d, want 1 (skip EPERM to ALLOW)", insn.Jt)
		}
		if insn.Jf != 0 {
			t.Errorf("clone flag JEQ: Jf=%d, want 0 (fall through to EPERM)", insn.Jf)
		}
		// Verify the target of Jt=1 is the allow return.
		jtTarget := flagCheckStartIdx + 2 + 1 + int(insn.Jt)
		if jtTarget != allowCloneIdx {
			t.Errorf("clone flag JEQ: Jt resolves to %d, want %d (allow clone)", jtTarget, allowCloneIdx)
		}
	}

	// --- Section 7: Return instructions ---
	assertInsn(t, prog, denyEPERMIdx, "ret EPERM (deny)", bpfRET|bpfK, 0, 0, errnoEPERM)
	assertInsn(t, prog, allowCloneIdx, "ret ALLOW (clone clean)", bpfRET|bpfK, 0, 0, seccompRetAllow)
	assertInsn(t, prog, denyENOSYSIdx, "ret ENOSYS (clone3)", bpfRET|bpfK, 0, 0, errnoENOSYS)

	// --- Section 8: No jump can land outside the program ---
	for i, insn := range prog {
		if insn.Code&0x07 == bpfJMP && insn.Code != bpfRET|bpfK {
			jtTarget := i + 1 + int(insn.Jt)
			jfTarget := i + 1 + int(insn.Jf)
			if jtTarget >= len(prog) {
				t.Errorf("instruction %d: Jt target %d out of bounds (len=%d)", i, jtTarget, len(prog))
			}
			if jfTarget >= len(prog) {
				t.Errorf("instruction %d: Jf target %d out of bounds (len=%d)", i, jfTarget, len(prog))
			}
		}
	}

	// --- Section 9: Verify all documented dangerous syscalls are present ---
	documentedSyscalls := map[string]uint32{
		"SYS_PTRACE":          unix.SYS_PTRACE,
		"SYS_MOUNT":           unix.SYS_MOUNT,
		"SYS_UMOUNT2":         unix.SYS_UMOUNT2,
		"SYS_REBOOT":          unix.SYS_REBOOT,
		"SYS_KEXEC_LOAD":      unix.SYS_KEXEC_LOAD,
		"SYS_INIT_MODULE":     unix.SYS_INIT_MODULE,
		"SYS_FINIT_MODULE":    unix.SYS_FINIT_MODULE,
		"SYS_DELETE_MODULE":   unix.SYS_DELETE_MODULE,
		"SYS_PIVOT_ROOT":      unix.SYS_PIVOT_ROOT,
		"SYS_CHROOT":          unix.SYS_CHROOT,
		"SYS_SWAPON":          unix.SYS_SWAPON,
		"SYS_SWAPOFF":         unix.SYS_SWAPOFF,
		"SYS_SETNS":           unix.SYS_SETNS,
		"SYS_UNSHARE":         unix.SYS_UNSHARE,
		"SYS_ACCT":            unix.SYS_ACCT,
		"SYS_SETTIMEOFDAY":    unix.SYS_SETTIMEOFDAY,
		"SYS_CLOCK_SETTIME":   unix.SYS_CLOCK_SETTIME,
		"SYS_ADJTIMEX":        unix.SYS_ADJTIMEX,
		"SYS_KEXEC_FILE_LOAD": 320,
	}
	presentSyscalls := make(map[uint32]bool)
	for _, nr := range deniedSyscalls {
		presentSyscalls[nr] = true
	}
	for name, nr := range documentedSyscalls {
		if !presentSyscalls[nr] {
			t.Errorf("documented dangerous syscall %s (%d) missing from denylist", name, nr)
		}
	}
}

// TestBuildSeccompFilterClone3Denied verifies clone3 is routed to ENOSYS,
// separate from the clone flag-check logic.
func TestBuildSeccompFilterClone3Denied(t *testing.T) {
	prog := buildSeccompFilter()

	// Find the clone3 comparison.
	found := false
	for i, insn := range prog {
		if insn.Code == bpfJMP|bpfJEQ|bpfK && insn.K == 435 {
			found = true
			target := i + 1 + int(insn.Jt)
			if target >= len(prog) {
				t.Fatalf("clone3 Jt target %d out of bounds", target)
			}
			// The target must be a RET with ENOSYS.
			ret := prog[target]
			if ret.Code != bpfRET|bpfK || ret.K != errnoENOSYS {
				t.Errorf("clone3 Jt target is not RET ENOSYS: code=%#x K=%#x", ret.Code, ret.K)
			}
			// clone3 must NOT go through clone's flag-check path.
			// Verify it does not target any LD/AND/JEQ instruction.
			if prog[target].Code != bpfRET|bpfK {
				t.Errorf("clone3 resolves to non-RET instruction at %d", target)
			}
			break
		}
	}
	if !found {
		t.Fatal("clone3 (syscall 435) not found in filter")
	}
}

// assertInsn checks a single BPF instruction at the given index.
func assertInsn(t *testing.T, prog []bpfInsn, idx int, name string, code uint16, jt, jf uint8, k uint32) {
	t.Helper()
	if idx >= len(prog) {
		t.Fatalf("%s: index %d out of bounds (len=%d)", name, idx, len(prog))
	}
	insn := prog[idx]
	if insn.Code != code {
		t.Errorf("%s [%d]: Code=%#x, want %#x", name, idx, insn.Code, code)
	}
	if insn.Jt != jt {
		t.Errorf("%s [%d]: Jt=%d, want %d", name, idx, insn.Jt, jt)
	}
	if insn.Jf != jf {
		t.Errorf("%s [%d]: Jf=%d, want %d", name, idx, insn.Jf, jf)
	}
	if insn.K != k {
		t.Errorf("%s [%d]: K=%#x, want %#x", name, idx, insn.K, k)
	}
}
