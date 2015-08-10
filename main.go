package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

// The following vars will be injected during the build process.
var (
	Version string
	GitSHA  string
)

const (
	hardcodedWarning = `
        WARNING: The 'boot2docker' command line interface is being officially deprecated.
	Users are expected to switch to Docker Machine (https://docs.docker.com/machine/) instead ASAP.
	The Docker Toolbox is the recommended way to install it: https://docker.com/toolbox/

`
	warningURL = "https://raw.githubusercontent.com/boot2docker/boot2docker-cli/master/DEPRECATION_WARNING"
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
		printDeprecationWarning()
		return cmdInit()
	case "up", "start", "boot", "resume":
		printDeprecationWarning()
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
	case "shellinit", "socket":
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
		// Version is now printed by the call to config()
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

func printDeprecationWarning() {
	var (
		warning string
	)

	// Try to get the warning from the Github raw URL.  If there's any
	// failure along the way, e.g. network, just fall back to the default
	// warning hardcoded in the source.
	resp, err := http.Get(warningURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		warning = hardcodedWarning
	} else {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			warning = hardcodedWarning
		} else {
			warning = string(body)
		}
	}

	fmt.Fprintln(os.Stderr, warning)
}
