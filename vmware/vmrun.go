package vmware

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

var (
	ErrMachineExist    = errors.New("machine already exists")
	ErrMachineNotExist = errors.New("machine does not exist")
	ErrVMRUNNotFound   = errors.New("VMRUN not found")
)

func vmrun(args ...string) error {
	cmd := exec.Command(cfg.VMRUN, args...)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Printf("executing: %v %v", cfg.VMRUN, strings.Join(args, " "))
	}
	if stdout := cmd.Run(); stdout != nil {
		if ee, ok := stdout.(*exec.Error); ok && ee == exec.ErrNotFound {
			return ErrVMRUNNotFound
		}
	}
	return nil
}

// Make a vmdk disk image with the given size (in MB).
func vdiskmanager(dest string, size uint) error {
	cmd := exec.Command(cfg.VDISKMAN, "-c", "-t", "0", "-s", fmt.Sprintf("%dMB", size), "-a", "lsilogic", dest)

	if stdout := cmd.Run(); stdout != nil {
		if ee, ok := stdout.(*exec.Error); ok && ee == exec.ErrNotFound {
			return ErrVMRUNNotFound
		}
	}
	return nil
}
