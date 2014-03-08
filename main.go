// This is the boot2docker management utilty.
package main

import (
	"os"

	// keep 3rd-party imports separate from stdlib with an empty line
	flag "github.com/ogier/pflag"
)

// The following vars will be injected during the build process.
var (
	Version string
	GitSHA  string
)

func main() {
	// os.Exit will terminate the program at the place of call without running
	// any deferred cleanup statements. It might cause unintended effects. To
	// be safe, we wrap the program in run() and only os.Exit() outside the
	// wrapper. Be careful not to indirectly trigger os.Exit() in the program,
	// notably via log.Fatal().
	os.Exit(run())
}

// Run the program and return exit code.
func run() int {
	flag.Usage = usageLong // make "-h" work similarly to "help"

	if err := config(); err != nil {
		errf("%s\n", err)
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
		outf("Client version: %s\nGit commit: %s\n", Version, GitSHA)
		return 0
	case "help":
		flag.Usage()
		return 0
	case "":
		usageShort()
		return 0
	default:
		errf("Unknown command %q\n", cmd)
		usageShort()
		return 1
	}
}

func usageShort() {
	errf("Usage: %s [<options>] {help|init|up|ssh|save|down|poweroff|reset|restart|status|info|delete|download|version} [<args>]\n", os.Args[0])

}

func usageLong() {
	// NOTE: the help message uses spaces, not tabs for indentation!
	errf(`boot2docker management utility.

Usage: %s [<options>] <command> [<args>]

Commands:
    init [<vm>]             Create a new boot2docker VM.
    up|start|boot [<vm>]    Start VM from any states.
    ssh                     Login to VM via SSH.
    save|suspend [<vm>]     Suspend VM and save state to disk.
    down|stop|halt [<vm>]   Gracefully shutdown the VM.
    restart [<vm>]          Gracefully reboot the VM.
    poweroff [<vm>]         Forcefully power off the VM (might corrupt disk image).
    reset [<vm>]            Forcefully power cycle the VM (might corrupt disk image).
    delete [<vm>]           Delete boot2docker VM and its disk image.
    info [<vm>]             Display detailed information of VM.
    status [<vm>]           Display current state of VM.
    download                Download boot2docker ISO image.
    version                 Display version information.

Options:
`, os.Args[0])
	flag.PrintDefaults()
}
