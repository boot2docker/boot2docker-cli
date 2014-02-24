// This is the boot2docker management utilty.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
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

var usageShort = fmt.Sprintf(`Usage: %s COMMAND {help|init|start|up|ssh|save|pause|stop|poweroff|reset|restart|status|info|delete|download} [vm]
`, os.Args[0])

var usageLong = fmt.Sprintf(`Usage: %s COMMAND [vm]

boot2docker management utility.

Commands:

    init            Create a new boot2docker VM.
    up|start|boot   Start the VM from any state.
    ssh             Login to VM.
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

func getCfgDir(name string) (string, error) {
	if b2dDir := os.Getenv("BOOT2DOCKER_DIR"); b2dDir != "" {
		return b2dDir, nil
	}

	// Unix
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, name), nil
	}

	// Windows
	for _, env := range []string{
		"APPDATA",
		"LOCALAPPDATA",
		"USERPROFILE", // let's try USERPROFILE only as a very last resort
	} {
		if val := os.Getenv(env); val != "" {
			return filepath.Join(val, "boot2docker"), nil
		}
	}
	// ok, we've tried everything reasonable - now let's go for CWD
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, name), nil
}

// Read configuration.
func config() (err error) {
	B2D.VBM = getenv("BOOT2DOCKER_VBM", "VBoxManage")
	B2D.SSH = getenv("BOOT2DOCKER_SSH", "ssh")
	B2D.VM = getenv("BOOT2DOCKER_VM", "boot2docker-vm")
	if B2D.Dir, err = getCfgDir(".boot2docker"); err != nil {
		return fmt.Errorf("failed to get current directory: %s", err)
	}

	B2D.ISO = getenv("BOOT2DOCKER_ISO", filepath.Join(B2D.Dir, "boot2docker.iso"))
	B2D.Disk = getenv("BOOT2DOCKER_DISK", filepath.Join(B2D.Dir, "boot2docker.vmdk"))

	if B2D.DiskSize, err = strconv.Atoi(getenv("BOOT2DOCKER_DISKSIZE", "20000")); err != nil {
		return fmt.Errorf("invalid BOOT2DOCKER_DISKSIZE: %s", err)
	}
	if B2D.DiskSize <= 0 {
		return fmt.Errorf("BOOT2DOCKER_DISKSIZE way too small")
	}
	if B2D.Memory, err = strconv.Atoi(getenv("BOOT2DOCKER_MEMORY", "1024")); err != nil {
		return fmt.Errorf("invalid BOOT2DOCKER_MEMORY: %s", err)
	}
	if B2D.Memory <= 0 {
		return fmt.Errorf("BOOT2DOCKER_MEMORY way too small")
	}
	if B2D.SSHPort, err = strconv.Atoi(getenv("BOOT2DOCKER_SSH_PORT", "2022")); err != nil {
		return fmt.Errorf("invalid BOOT2DOCKER_SSH_PORT: %s", err)
	}
	if B2D.SSHPort <= 0 {
		return fmt.Errorf("invalid BOOT2DOCKER_SSH_PORT: must be in the range of 1--65535; got %d", B2D.SSHPort)
	}
	if B2D.DockerPort, err = strconv.Atoi(getenv("BOOT2DOCKER_DOCKER_PORT", "4243")); err != nil {
		return fmt.Errorf("invalid BOOT2DOCKER_DOCKER_PORT: %s", err)
	}
	if B2D.DockerPort <= 0 {
		return fmt.Errorf("invalid BOOT2DOCKER_DOCKER_PORT: must be in the range of 1--65535; got %d", B2D.DockerPort)
	}

	// TODO maybe allow flags to override ENV vars?
	flag.Parse()
	if vm := flag.Arg(1); vm != "" {
		B2D.VM = vm
	}
	return
}

func run() int {
	if err := config(); err != nil {
		logf("%s", err)
		return 1
	}

	if _, err := exec.LookPath(B2D.VBM); err != nil {
		logf("failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	}

	switch flag.Arg(0) {
	case "download":
		return cmdDownload()
	case "init":
		return cmdInit()
	case "start", "up", "boot", "resume":
		return cmdStart()
	case "ssh":
		return cmdSSH()
	case "save", "suspend":
		return cmdSave()
	case "pause":
		return cmdPause()
	case "halt", "down", "stop":
		return cmdStop()
	case "poweroff":
		return cmdPoweroff()
	case "restart":
		return cmdRestart()
	case "reset":
		return cmdReset()
	case "info":
		return cmdInfo()
	case "status":
		return cmdStatus()
	case "help":
		logf(usageLong)
		return 0
	case "delete":
		return cmdDelete()
	case "":
		logf(usageShort)
		return 0
	default:
		logf("Unknown command '%s'",flag.Arg(0))
		logf(usageShort)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
