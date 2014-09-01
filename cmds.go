package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	flag "github.com/docker/docker/pkg/mflag"

	_ "github.com/boot2docker/boot2docker-cli/dummy"
	_ "github.com/boot2docker/boot2docker-cli/virtualbox"

	"github.com/boot2docker/boot2docker-cli/driver"
)

// Initialize the boot2docker VM from scratch.
func cmdInit() error {
	B2D.Init = false
	_, err := driver.GetMachine(&B2D)
	if err == nil {
		fmt.Printf("Virtual machine %s already exists\n", B2D.VM)
		return nil
	}

	if _, err := os.Stat(B2D.ISO); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("Failed to open ISO image %q: %s", B2D.ISO, err)
		}

		if err := cmdDownload(); err != nil {
			return err
		}
	}

	if _, err := os.Stat(B2D.SSHKey); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("Something wrong with SSH Key file %q: %s", B2D.SSHKey, err)
		}

		cmd := exec.Command(B2D.SSHGen, "-t", "rsa", "-N", "", "-f", B2D.SSHKey)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if B2D.Verbose {
			cmd.Stderr = os.Stderr
			fmt.Printf("executing: %v %v\n", cmd.Path, strings.Join(cmd.Args, " "))
		}

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("Error generating new SSH Key into %s: %s", B2D.SSHKey, err)
		}
	}
	//TODO: print a ~/.ssh/config entry for our b2d connection that the user can c&p

	B2D.Init = true
	_, err = driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to initialize machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Bring up the VM from all possible states.
func cmdUp() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Start(); err != nil {
		return fmt.Errorf("Failed to start machine %q: %s", B2D.VM, err)
	}

	if err := m.Refresh(); err != nil {
		return fmt.Errorf("Failed to start machine %q: %s", B2D.VM, err)
	}
	if m.GetState() != driver.Running {
		return fmt.Errorf("Failed to start machine %q (run again with -v for details)", B2D.VM)
	}

	fmt.Println("Waiting for VM and Docker daemon to start...")
	//give the VM a little time to start, so we don't kill the Serial Pipe/Socket
	time.Sleep(600 * time.Millisecond)
	natSSH := fmt.Sprintf("localhost:%d", m.GetSSHPort())
	IP := ""
	for i := 1; i < 30; i++ {
		print(".")
		if B2D.Serial && runtime.GOOS != "windows" {
			if IP, err = RequestIPFromSerialPort(m.GetSerialFile()); err == nil {
				break
			}
		}
		if err := read(natSSH, 1, 300*time.Millisecond); err == nil {
			if IP, err = RequestIPFromSSH(m); err == nil {
				break
			}
		}
	}
	if B2D.Verbose {
		fmt.Printf("VM Host-only IP address: %s", IP)
		fmt.Printf("\nWaiting for Docker daemon to start...\n")
	}

	time.Sleep(300 * time.Millisecond)
	socket := ""
	for i := 1; i < 30; i++ {
		print(".")
		if socket, err = RequestSocketFromSSH(m); err == nil {
			break
		}
		if B2D.Verbose {
			fmt.Printf("Error requesting socket: %s\n", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	fmt.Printf("\nStarted.\n")

	if socket == "" {
		// lets try one more time
		time.Sleep(600 * time.Millisecond)
		fmt.Printf("  Trying to get Docker socket one more time\n")

		if socket, err = RequestSocketFromSSH(m); err != nil {
			fmt.Printf("Error requesting socket: %s\n", err)
		}
	}
	// Copying the certs here - someone might have have written a Windows API client.
	certPath, err := RequestCertsUsingSSH(m)
	if err != nil && B2D.Verbose {
		// These errors are not fatal
		fmt.Fprintf(os.Stderr, "Error copying Certificates: %s\n", err)
	}
	switch runtime.GOOS {
	case "windows":
		fmt.Printf("Docker client does not run on Windows for now. Please use\n")
		fmt.Printf("    \"%s\" ssh\n", os.Args[0])
		fmt.Printf("to SSH into the VM instead.\n")
	default:
		if socket == "" {
			fmt.Fprintf(os.Stderr, "Auto detection of the VM's Docker socket failed.\n")
			fmt.Fprintf(os.Stderr, "Please run `boot2docker -v up` to diagnose.\n")
		} else {
			// Check if $DOCKER_HOST ENV var is properly configured.
			if os.Getenv("DOCKER_HOST") != socket || os.Getenv("DOCKER_CERT_PATH") != certPath {
				fmt.Printf("\nTo connect the Docker client to the Docker daemon, please set:\n")
				printExport(socket, certPath)
			} else {
				fmt.Printf("Your DOCKER_HOST env variable is already set correctly.\n")
			}
		}
	}
	fmt.Printf("\n")
	return nil
}

// Give the user the exact command to run to set the env.
func cmdShellInit() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}

	if m.GetState() != driver.Running {
		return fmt.Errorf("VM %q is not running.", B2D.VM)
	}

	socket, err := RequestSocketFromSSH(m)
	if err != nil {
		return fmt.Errorf("Error requesting socket: %s\n", err)
	}

	certPath, err := RequestCertsUsingSSH(m)
	if err != nil && B2D.Verbose {
		// These errors are not fatal
		fmt.Fprintf(os.Stderr, "Error copying Certificates: %s\n", err)
	}
	printExport(socket, certPath)

	return nil
}

func printExport(socket, certPath string) {
	fmt.Printf("    export DOCKER_HOST=%s\n", socket)
	if certPath == "" {
		if os.Getenv("DOCKER_CERT_PATH") != "" {
			fmt.Println("    unset DOCKER_CERT_PATH")
		}
	} else {
		// Assume Docker 1.2.0 with TLS on...
		fmt.Printf("    export DOCKER_CERT_PATH=%s\n", certPath)
	}
}

// Tell the user the config (and later let them set it?)
func cmdConfig(flags *flag.FlagSet) error {
	dir, err := cfgDir(".boot2docker")
	if err != nil {
		return fmt.Errorf("Error working out Profile file location: %s\n", err)
	}
	filename := cfgFilename(dir)
	fmt.Fprintf(os.Stderr, "boot2docker profile filename: %s\n", filename)
	fmt.Println(printConfig(flags))
	return nil
}

// Suspend and save the current state of VM on disk.
func cmdSave() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s\n", B2D.VM, err)
	}
	if err := m.Save(); err != nil {
		return fmt.Errorf("Failed to save machine %q: %s\n", B2D.VM, err)
	}
	return nil
}

// Gracefully stop the VM by sending ACPI shutdown signal.
func cmdStop() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Stop(); err != nil {
		return fmt.Errorf("Failed to stop machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Forcefully power off the VM (equivalent to unplug power). Might corrupt disk
// image.
func cmdPoweroff() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Poweroff(); err != nil {
		return fmt.Errorf("Failed to poweroff machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Upgrade the boot2docker ISO - preserving server state
func cmdUpgrade() error {
	m, err := driver.GetMachine(&B2D)
	if err == nil && m.GetState() == driver.Running {
		// Windows won't let us move the ISO aside while it's in use
		if cmdStop() == nil && cmdDownload() == nil {
			return cmdUp()
		} else {
			return nil
		}
	} else {
		return cmdDownload()
	}
}

// Gracefully stop and then start the VM.
func cmdRestart() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Restart(); err != nil {
		return fmt.Errorf("Failed to restart machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Forcefully reset (equivalent to cold boot) the VM. Might corrupt disk image.
func cmdReset() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Reset(); err != nil {
		return fmt.Errorf("Failed to reset machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Delete the VM and associated disk image.
func cmdDelete() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		if err == driver.ErrMachineNotExist {
			return fmt.Errorf("Machine %q does not exist.", B2D.VM)
		}
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Delete(); err != nil {
		return fmt.Errorf("Failed to delete machine %q: %s", B2D.VM, err)
	}
	return nil
}

// Show detailed info of the VM.
func cmdInfo() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(m); err != nil {
		return fmt.Errorf("Failed to encode machine %q info: %s", B2D.VM, err)
	}
	return nil
}

// Show the current state of the VM.
func cmdStatus() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	fmt.Println(m.GetState())
	return nil
}

// tell the User the Docker socket to connect to
func cmdSocket() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}

	if m.GetState() != driver.Running {
		return fmt.Errorf("VM %q is not running.", B2D.VM)
	}

	socket, err := RequestSocketFromSSH(m)
	if err != nil {
		return fmt.Errorf("Error requesting socket: %s\n", err)
	}

	fmt.Fprintf(os.Stderr, "\n\t export DOCKER_HOST=")
	fmt.Printf("%s", socket)
	fmt.Fprintf(os.Stderr, "\n\n")

	return nil
}

// Call the external SSH command to login into boot2docker VM.
func cmdSSH() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}

	if m.GetState() != driver.Running {
		return fmt.Errorf("VM %q is not running.", B2D.VM)
	}

	// find the ssh cmd string and then pass any remaining strings to ssh
	// TODO: it's a shame to repeat the same code as in config.go, but I
	//       didn't find a way to share the unsharable without more rework
	i := 1
	for i < len(os.Args) && os.Args[i-1] != "ssh" {
		i++
	}

	if err := cmdInteractive(m, os.Args[i:]...); err != nil {
		return fmt.Errorf("%s", err)
	}
	return nil
}

func cmdIP() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}

	if m.GetState() != driver.Running {
		return fmt.Errorf("VM %q is not running.", B2D.VM)
	}

	IP := ""
	if B2D.Serial {
		if runtime.GOOS != "windows" {
			if IP, err = RequestIPFromSerialPort(m.GetSerialFile()); err != nil {
				if B2D.Verbose {
					fmt.Printf("Error getting IP via Serial: %s\n", err)
				}
			}
		}
	}

	if IP == "" {
		if IP, err = RequestIPFromSSH(m); err != nil {
			if B2D.Verbose {
				fmt.Printf("Error getting IP via SSH: %s\n", err)
			}
		}
	}
	if IP != "" {
		fmt.Fprintf(os.Stderr, "\nThe VM's Host only interface IP address is: ")
		fmt.Printf("%s", IP)
		fmt.Fprintf(os.Stderr, "\n\n")
	} else {
		fmt.Fprintf(os.Stderr, "\nFailed to get VM Host only IP address.\n")
		fmt.Fprintf(os.Stderr, "\tWas the VM initilized using boot2docker?\n")
	}
	return nil
}

// Download the boot2docker ISO image.
func cmdDownload() error {
	fmt.Println("Downloading boot2docker ISO image...")
	url := "https://api.github.com/repos/boot2docker/boot2docker/releases"
	tag, err := getLatestReleaseName(url)
	if err != nil {
		return fmt.Errorf("Failed to get latest release: %s", err)
	}
	fmt.Printf("Latest release is %s\n", tag)

	url = fmt.Sprintf("https://github.com/boot2docker/boot2docker/releases/download/%s/boot2docker.iso", tag)
	if err := download(B2D.ISO, url); err != nil {
		return fmt.Errorf("Failed to download ISO image: %s", err)
	}
	fmt.Printf("Success: downloaded %s\n\tto %s\n", url, B2D.ISO)
	return nil
}
