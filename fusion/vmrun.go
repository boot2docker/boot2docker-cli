package fusion

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

var (
	ErrMachineExist    = errors.New("machine already exists")
	ErrMachineNotExist = errors.New("machine does not exist")
	ErrVMRUNNotFound   = errors.New("VMRUN not found")
)

func vmrun(args ...string) (string, string, error) {
	cmd := exec.Command(cfg.VMRUN, args...)
	if verbose {
		log.Printf("executing: %v %v", cfg.VMRUN, strings.Join(args, " "))
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.Error); ok && ee == exec.ErrNotFound {
			err = ErrVMRUNNotFound
		}
	}

	return stdout.String(), stderr.String(), err
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
