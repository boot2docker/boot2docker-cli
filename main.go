package main

import (
	vbx "github.com/boot2docker/boot2docker-cli/virtualbox"
	"os"
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
	// notably via log.Fatal() and on flag.Parse() where the default behavior
	// is ExitOnError.
	os.Exit(run())
}

// Run the program and return exit code.
func run() int {
	flags, err := config()
	if err != nil {
		errf("config error: %v\n", err)
		return 1
	}

	stdout := os.Stdout

	switch cmd := flags.Arg(0); cmd {
	case "download":
		return cmdDownload()
	case "config", "cfg":
		return cmdConfig()
	case "init":
		return cmdInit()
	case "up", "start", "boot", "resume":
		return cmdUp()
	case "save", "suspend":
		return cmdSave()
	case "down", "halt", "stop":
		return cmdStop()
	case "poweroff":
		return cmdPoweroff()
	case "restart":
		return cmdRestart()
	case "reset":
		return cmdReset()
	case "delete", "destroy":
		return cmdDelete()
	case "info":
		return cmdInfo()
	case "status":
		return cmdStatus()
	case "ssh":
		return cmdSSH()
	case "ip":
		return cmdIP()
	case "shellsetup":
		m, err := vbx.GetMachine(B2D.VM)
		if err != nil {
			logf("Failed to get machine %q: %s", B2D.VM, err)
			return 2
		}
		return cmdShellSetup(m, stdout)
	case "version":
		outf("Client version: %s\nGit commit: %s\n", Version, GitSHA)
		return 0
	case "help":
		flags.Usage()
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
