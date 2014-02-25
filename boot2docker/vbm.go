package main

import (
	"fmt"
	"io/ioutil"
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

// Make a boot2docker VM disk image.
func makeDiskImage(dest string, size int) error {
	// Create the dest dir.
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	d1 := []byte("boot2docker, please format-me")
	if err := ioutil.WriteFile(dest+"_RAW", d1, 0644); err != nil {
		return err
	}

	cmd := exec.Command(B2D.VBM, "convertfromraw", "stdin", dest, fmt.Sprintf("%d", size*1024*1024), "--format", "VMDK")

	return cmd.Run()
}
