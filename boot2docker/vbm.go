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

	if err := vbm("createhd",
		"--format", "VMDK",
		"--filename", dest,
		"--size", fmt.Sprintf("%d", size),
	); err != nil {
		return err
	}

	tmpRaw, err := makeRawImage()
	if err != nil {
		return err
	}
	defer os.Remove(tmpRaw)

	tmpVMDK := fmt.Sprintf("%s.tmp", dest)
	if err := vbm("convertfromraw", tmpRaw, tmpVMDK, "--format", "VMDK"); err != nil {
		return err
	}
	defer os.Remove(tmpVMDK) // doesn't hurt if this fails

	if err := vbm("clonehd", tmpVMDK, dest, "--existing"); err != nil {
		return err
	}
	vbm("closemedium", "disk", tmpVMDK) // doesn't hurt if this fails
	return nil
}

// Make the raw image to be converted to VMDK image.
func makeRawImage() (string, error) {
	f, err := ioutil.TempFile("", "boot2docker-")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Truncate(5 * 1024 * 1024); err != nil {
		os.Remove(name)
		return "", err
	}
	if _, err = f.WriteString("boot2docker, please format-me"); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
