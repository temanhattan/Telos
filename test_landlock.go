package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	LANDLOCK_ACCESS_FS_EXECUTE = 1 << 0
	LANDLOCK_ACCESS_FS_READ_FILE = 1 << 2
	LANDLOCK_ACCESS_FS_READ_DIR = 1 << 3
)

func main() {
	abi, _, _ := syscall.Syscall(444, 0, 0, 1) // SYS_LANDLOCK_CREATE_RULESET
	if abi <= 0 {
		fmt.Println("No Landlock")
		return
	}
	
	type landlockAttr struct {
		AllowedAccessFS uint64
		AllowedAccessNet uint64
	}
	attr := landlockAttr{AllowedAccessFS: LANDLOCK_ACCESS_FS_EXECUTE | LANDLOCK_ACCESS_FS_READ_FILE | LANDLOCK_ACCESS_FS_READ_DIR}
	fd, _, err := syscall.Syscall(444, uintptr(unsafe.Pointer(&attr)), 16, 0)
	if err != 0 {
		fmt.Println("create", err)
		return
	}
	
	addRule := func(path string, access uint64) {
		pfd, _ := syscall.Open(path, syscall.O_PATH|syscall.O_CLOEXEC, 0)
		defer syscall.Close(pfd)
		type landlockPathBeneath struct {
			AllowedAccess uint64
			ParentFD      int32
			_             [4]byte
		}
		rule := landlockPathBeneath{AllowedAccess: access, ParentFD: int32(pfd)}
		syscall.Syscall6(445, fd, 1, uintptr(unsafe.Pointer(&rule)), 0, 0, 0)
	}

	addRule("/bin/ls", LANDLOCK_ACCESS_FS_EXECUTE | LANDLOCK_ACCESS_FS_READ_FILE)
	// Add read to libraries
	addRule("/lib", LANDLOCK_ACCESS_FS_READ_FILE | LANDLOCK_ACCESS_FS_READ_DIR)
	addRule("/lib64", LANDLOCK_ACCESS_FS_READ_FILE | LANDLOCK_ACCESS_FS_READ_DIR) // NO EXECUTE
	addRule("/usr", LANDLOCK_ACCESS_FS_READ_FILE | LANDLOCK_ACCESS_FS_READ_DIR)
	
	if err := syscall.Prctl(syscall.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		fmt.Println("prctl", err)
		return
	}
	if _, _, e := syscall.Syscall(446, fd, 0, 0); e != 0 {
		fmt.Println("restrict", e)
		return
	}
	
	cmd := exec.Command("/bin/ls")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err2 := cmd.Run()
	fmt.Println("Exit:", err2)
}
