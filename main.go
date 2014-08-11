package main

import (
	"os"
	"fmt"
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
	err := run()
	if err != nil {
		os.Exit(0)
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

// Run the program and return exit code.
func run() error {
	flags, err := config()
	if err != nil {
		return fmt.Errorf("config error: %v\n", err)
	}

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
	case "upgrade":
		return cmdUpgrade()
	case "version":
		fmt.Printf("Client version: %s\nGit commit: %s\n", Version, GitSHA)
		return nil
	case "help":
		flags.Usage()
		return nil
	case "":
		usageShort()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n", cmd)
		usageShort()
		return fmt.Errorf("Unknown command %q\n", cmd)
	}
}
