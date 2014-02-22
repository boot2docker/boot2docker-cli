// This is the boot2docker management utilty.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
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

func getCfgDir(name string) string {
	if b2dDir := os.Getenv("BOOT2DOCKER_DIR"); b2dDir != "" {
		return b2dDir
	}

	// Unix
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, name)
	}

	// Windows
	for _, env := range []string{
		"APPDATA",
		"LOCALAPPDATA",
		"USERPROFILE", // let's try USERPROFILE only as a very last resort
	} {
		if val := os.Getenv(env); val != "" {
			return filepath.Join(val, "boot2docker")
		}
	}
	// ok, we've tried everything reasonable - now let's go for CWD
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error getting current directory: %s", err)
	}
	return filepath.Join(cwd, name)
}

func init() {
	B2D.VBM = getenv("BOOT2DOCKER_VBM", "VBoxManage")
	B2D.SSH = getenv("BOOT2DOCKER_SSH", "ssh")
	B2D.VM = getenv("BOOT2DOCKER_VM", "boot2docker-vm")
	B2D.Dir = getCfgDir(".boot2docker")
	B2D.ISO = getenv("BOOT2DOCKER_ISO", filepath.Join(B2D.Dir, "boot2docker.iso"))
	B2D.Disk = getenv("BOOT2DOCKER_DISK", filepath.Join(B2D.Dir, "boot2docker.vmdk"))

	var err error
	if B2D.DiskSize, err = strconv.Atoi(getenv("BOOT2DOCKER_DISKSIZE", "20000")); err != nil {
		log.Fatalf("Invalid BOOT2DOCKER_DISKSIZE: %s", err)
	}
	if B2D.DiskSize <= 0 {
		log.Fatalf("BOOT2DOCKER_DISKSIZE way too small.")
	}
	if B2D.Memory, err = strconv.Atoi(getenv("BOOT2DOCKER_MEMORY", "1024")); err != nil {
		log.Fatalf("Invalid BOOT2DOCKER_MEMORY: %s", err)
	}
	if B2D.Memory <= 0 {
		log.Fatalf("BOOT2DOCKER_MEMORY way too small.")
	}
	if B2D.SSHPort, err = strconv.Atoi(getenv("BOOT2DOCKER_SSH_PORT", "2022")); err != nil {
		log.Fatalf("Invalid BOOT2DOCKER_SSH_PORT: %s", err)
	}
	if B2D.SSHPort <= 0 {
		log.Fatalf("Invalid BOOT2DOCKER_SSH_PORT: must be in the range of 1--65535: got %d", B2D.SSHPort)
	}
	if B2D.DockerPort, err = strconv.Atoi(getenv("BOOT2DOCKER_DOCKER_PORT", "4243")); err != nil {
		log.Fatalf("Invalid BOOT2DOCKER_DOCKER_PORT: %s", err)
	}
	if B2D.DockerPort <= 0 {
		log.Fatalf("Invalid BOOT2DOCKER_DOCKER_PORT: must be in the range of 1--65535: got %d", B2D.DockerPort)
	}

	// TODO maybe allow flags to override ENV vars?
	flag.Parse()
}

func main() {
	if vm := flag.Arg(1); vm != "" {
		B2D.VM = vm
	}

	// TODO maybe use reflect here?
	switch flag.Arg(0) { // choose subcommand
	case "download":
		cmdDownload()
	case "init":
		cmdInit()
	case "start", "up", "boot", "resume":
		cmdStart()
	case "ssh":
		cmdSSH()
	case "save", "suspend":
		cmdSave()
	case "pause":
		cmdPause()
	case "halt", "down", "stop":
		cmdStop()
	case "poweroff":
		cmdPoweroff()
	case "restart":
		cmdRestart()
	case "reset":
		cmdReset()
	case "info":
		cmdInfo()
	case "status":
		cmdStatus()
	case "delete":
		cmdDelete()
	default:
		fmt.Printf(`Usage: %s COMMAND [vm]

boot2docker management utility.

Commands:

    init            Create a new boot2docker VM.
    up|start|boot   Start the VM from any state.
    save|suspend    Suspend the VM (saving running state to disk).
    down|stop|halt  Gracefully shutdown the VM.
    restart         Gracefully reboot the VM.
    poweroff        Forcefully shutdown the VM (might cause disk corruption).
    reset           Forcefully reboot the VM (might cause disk corruption).
    delete          Delete the boot2docker VM and its disk image.
    download        Download the boot2docker ISO image.
    info            Display the detailed information of the VM
    status          Display the current state of the VM.

`, os.Args[0])
	}
}
