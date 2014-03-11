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

// TODO: delete the hostonlyif and dhcpserver when we delete the vm! (need to
// reference count to make sure there are no other vms relying on them)

// Get or create the hostonly network interface
func getHostOnlyNetworkInterface() (string, error) {
	// Check if the interface exists.
	out, err := exec.Command(B2D.VBM, "list", "hostonlyifs").Output()
	if err != nil {
		return "", err
	}
	lists := regexp.MustCompile(`(?m)^(Name|IPAddress|VBoxNetworkName):\s+(.+?)\r?$`).FindAllSubmatch(out, -1)
	var ifname string
	index := 0

	for ifname == "" && len(lists) > index {
		if string(lists[index+1][2]) == B2D.HostIP {
			//test to see that the dhcpserver is the same too
			out, err := exec.Command(B2D.VBM, "list", "dhcpservers").Output()
			if err != nil {
				return "", err
			}
			dhcp := regexp.MustCompile(`(?m)^(NetworkName|IP|NetworkMask|lowerIPAddress|upperIPAddress|Enabled):\s+(.+?)\r?$`).FindAllSubmatch(out, -1)
			i := 0

			for ifname == "" && len(dhcp) > i {
				info := map[string]string{}
				for id := 0; id < 6; id++ {
					info[string(dhcp[i][1])] = string(dhcp[i][2])
					i++
				}

				if string(info["NetworkName"]) == string(lists[index+2][2]) &&
					info["IP"] == B2D.DHCPIP &&
					info["NetworkMask"] == B2D.NetworkMask &&
					info["lowerIPAddress"] == B2D.LowerIPAddress &&
					info["upperIPAddress"] == B2D.UpperIPAddress &&
					info["Enabled"] == B2D.DHCPEnabled {
					ifname = string(lists[index][2])
					logf("Reusing hostonly network interface %s\n", ifname)
				}
			}
		}
		index = index + 3
	}

	if ifname == "" {
		//create it all fresh
		logf("Creating a new hostonly network interface\n")
		out, err = exec.Command(B2D.VBM, "hostonlyif", "create").Output()
		if err != nil {
			return "", err
		}
		groups := regexp.MustCompile(`(?m)^Interface '(.+)' was successfully created`).FindSubmatch(out)
		if len(groups) < 2 {
			return "", err
		}
		ifname = string(groups[1])
		out, err = exec.Command(B2D.VBM, "dhcpserver", "add",
			"--ifname", ifname,
			"--ip", B2D.DHCPIP,
			"--netmask", B2D.NetworkMask,
			"--lowerip", B2D.LowerIPAddress,
			"--upperip", B2D.UpperIPAddress,
			"--enable",
		).Output()
		if err != nil {
			return "", err
		}
		out, err = exec.Command(B2D.VBM, "hostonlyif", "ipconfig", ifname,
			"--ip", B2D.HostIP,
			"--netmask", B2D.NetworkMask,
		).Output()
		if err != nil {
			return "", err
		}
	}
	return ifname, nil
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

// Get the VirtualBox base folder of the VM.
func basefolder(vm string) string {
	out, err := exec.Command(B2D.VBM, "showvminfo", vm, "--machinereadable").Output()
	if err != nil {
		return ""
	}
	groups := regexp.MustCompile(`(?m)^CfgFile="(.+)"\r?$`).FindSubmatch(out)
	if len(groups) < 2 {
		return ""
	}
	return filepath.Dir(string(groups[1]))
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
