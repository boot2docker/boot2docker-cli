package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
)

// B2D reprents boot2docker config.
var B2D struct {
	VBM        string // VirtualBox management utility
	SSH        string // SSH client executable
	VM         string // virtual machine name
	Dir        string // boot2docker directory
	ISO        string // boot2docker ISO image path
	Disk       string // VM disk image path
	DiskSize   int    // VM disk image size (MB)
	Memory     int    // VM memory size (MB)
	SSHPort    int    // host SSH port (forward to port 22 in VM)
	DockerPort int    // host Docker port (forward to port 4243 in VM)
}

// Read configuration.
func config() (err error) {

	if B2D.Dir, err = getCfgDir(); err != nil {
		return fmt.Errorf("failed to get current directory: %s", err)
	}
	cfgi, err := getConfigfile()

	B2D.VBM = cfgi.Get("", "VBM", "VBoxManage")
	B2D.SSH = cfgi.Get("", "BOOT2DOCKER_SSH", "ssh")
	B2D.VM = cfgi.Get("", "VM_NAME", "boot2docker-vm")

	B2D.ISO = cfgi.Get("", "BOOT2DOCKER_ISO", filepath.Join(B2D.Dir, "boot2docker.iso"))
	B2D.Disk = cfgi.Get("", "VM_DISK", filepath.Join(B2D.Dir, "boot2docker.vmdk"))

	if B2D.DiskSize, err = strconv.Atoi(cfgi.Get("", "VM_DISK_SIZE", "20000")); err != nil {
		return fmt.Errorf("invalid VM_DISK_SIZE: %s", err)
	}
	if B2D.DiskSize <= 0 {
		return fmt.Errorf("VM_DISK_SIZE way too small")
	}
	if B2D.Memory, err = strconv.Atoi(cfgi.Get("", "VM_MEM", "1024")); err != nil {
		return fmt.Errorf("invalid VM_MEM: %s", err)
	}
	if B2D.Memory <= 0 {
		return fmt.Errorf("VM_MEM way too small")
	}
	if B2D.SSHPort, err = strconv.Atoi(cfgi.Get("", "SSH_HOST_PORT", "2022")); err != nil {
		return fmt.Errorf("invalid SSH_HOST_PORT: %s", err)
	}
	if B2D.SSHPort <= 0 {
		return fmt.Errorf("invalid SSH_HOST_PORT: must be in the range of 1--65535; got %d", B2D.SSHPort)
	}
	if B2D.DockerPort, err = strconv.Atoi(cfgi.Get("", "DOCKER_PORT", "4243")); err != nil {
		return fmt.Errorf("invalid DOCKER_PORT: %s", err)
	}
	if B2D.DockerPort <= 0 {
		return fmt.Errorf("invalid DOCKER_PORT: must be in the range of 1--65535; got %d", B2D.DockerPort)
	}

	// TODO maybe allow flags to override ENV vars?
	flag.Parse()
	if vm := flag.Arg(1); vm != "" {
		B2D.VM = vm
	}
	return
}
