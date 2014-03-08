// This is the boot2docker management utilty.
package main

import (
	"flag"
	"fmt"
	"os"
)

// The following will be injected during the build process.
var (
	Version     string
	GitSHA      string
	verboseFlag = flag.Bool("v", false, "verbose flag to show commands as they execute.")
)

var usageShort = fmt.Sprintf(`Usage: %s [-v] {help|init|start|up|ssh|save|pause|stop|poweroff|reset|restart|status|info|delete|download|version} [<vm>]
`, os.Args[0])

// NOTE: the help message uses spaces, not tabs for indentation!
var usageLong = fmt.Sprintf(`Usage: %s [-v] <command> [<vm>]

boot2docker management utility.

Flags:
    -v              Verbose flag to show commands as they execute.

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
    version         Display version information.

`, os.Args[0])

func run() int {
	if err := config(); err != nil {
		logf("%s", err)
		return 1
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
	case "delete":
		return cmdDelete()
	case "version":
		fmt.Println("Client version:", Version)
		fmt.Println("Git commit:", GitSHA)
		return 0
	case "help":
		logf(usageLong)
		return 0
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
	flag.Parse()
	os.Exit(run())
}
