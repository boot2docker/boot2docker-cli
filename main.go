package main

import (
	"fmt"
	"os"
)

// The following vars will be injected during the build process.
var (
	Version string
	GitSHA  string
)

type unknownCommandError struct {
	cmd string
}

func (e unknownCommandError) Error() string {
	return fmt.Sprintf("Unknown command: %s", e.cmd)
}

func main() {
	// os.Exit will terminate the program at the place of call without running
	// any deferred cleanup statements. It might cause unintended effects. To
	// be safe, we wrap the program in run() and only os.Exit() outside the
	// wrapper. Be careful not to indirectly trigger os.Exit() in the program,
	// notably via log.Fatal() and on flag.Parse() where the default behavior
	// is ExitOnError.
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error in run: %v\n", err)
		if _, ok := err.(unknownCommandError); ok {
			usageShort()
		}
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
	case "socket":
		return cmdSocket()
	case "share":
		return cmdShare()
	case "shellinit":
		return cmdShellInit()
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
		return unknownCommandError{cmd: cmd}
	}
}
