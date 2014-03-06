package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

// Convenient function to exec a command.
func cmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Convenient function to launch VBoxManage.
func vbm(args ...string) error {
	return cmd(B2D.VBM, args...)
}

// Get the state of a VM.
func status(vm string) vmState {
	// Check if the VM exists.
	out, err := exec.Command(B2D.VBM, "list", "vms").Output()
	if err != nil {
		if err.(*exec.Error).Err == exec.ErrNotFound {
			return vmVBMNotFound
		}
		return vmUnknown
	}
	found, err := regexp.Match(fmt.Sprintf(`(?m)^"%s"`, regexp.QuoteMeta(vm)), out)
	if err != nil {
		return vmUnknown
	}
	if !found {
		return vmUnregistered
	}

	if out, err = exec.Command(B2D.VBM, "showvminfo", vm, "--machinereadable").Output(); err != nil {
		if err.(*exec.Error).Err == exec.ErrNotFound {
			return vmVBMNotFound
		}
		return vmUnknown
	}
	groups := regexp.MustCompile(`(?m)^VMState="(\w+)"\r?$`).FindSubmatch(out)
	if len(groups) < 2 {
		return vmUnknown
	}
	switch state := vmState(groups[1]); state {
	case vmRunning, vmPaused, vmSaved, vmPoweroff, vmAborted:
		return state
	default:
		return vmUnknown
	}
}

// Make a boot2docker VM disk image with the given size (in MB).
func makeDiskImage(dest string, size uint) error {
	// Create the dest dir.
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Convert a raw image from stdin to the dest VMDK image.
	sizeBytes := int64(size) * 1024 * 1024 // usually won't fit in 32-bit int
	cmd := exec.Command(B2D.VBM, "convertfromraw", "stdin", dest,
		fmt.Sprintf("%d", sizeBytes), "--format", "VMDK")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	// Fill in the magic string so boot2docker VM will detect this and format
	// the disk upon first boot.
	magic := []byte("boot2docker, please format-me")
	if _, err := stdin.Write(magic); err != nil {
		return err
	}
	// The total number of bytes written to stdin must match sizeBytes, or
	// VBoxManage.exe on Windows will fail.
	if err := zeroFill(stdin, sizeBytes-int64(len(magic))); err != nil {
		return err
	}
	// cmd won't exit until the stdin is closed.
	if err := stdin.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

// Write n zero bytes into w.
func zeroFill(w io.Writer, n int64) (err error) {
	const blocksize = 32 * 1024
	zeros := make([]byte, blocksize)
	var k int
	for n > 0 {
		if n > blocksize {
			k, err = w.Write(zeros)
		} else {
			k, err = w.Write(zeros[:n])
		}
		if err != nil {
			return
		}
		n -= int64(k)
	}
	return
}
