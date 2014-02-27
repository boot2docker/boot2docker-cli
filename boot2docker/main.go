// This is the boot2docker management utilty.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

var usageShort = fmt.Sprintf(`Usage: %s {help|init|start|up|ssh|save|pause|stop|poweroff|reset|restart|status|info|delete|download} [<vm>]
`, os.Args[0])

var usageLong = fmt.Sprintf(`Usage: %s <command> [<vm>]

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

func run() int {
	flag.Parse()

	vm := flag.Arg(1)
	if err := config(vm); err != nil {
		logf("%s", err)
		return 1
	}

	if _, err := exec.LookPath(B2D.VBM); err != nil {
		logf("failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	}

	switch cmd := flag.Arg(0); cmd {
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
		logf("Unknown command '%s'", cmd)
		logf(usageShort)
		return 1
	}
}

func main() {
	os.Exit(run())
}
