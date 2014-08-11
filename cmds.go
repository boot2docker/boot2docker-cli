package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	_ "github.com/boot2docker/boot2docker-cli/dummy"
	_ "github.com/boot2docker/boot2docker-cli/virtualbox"

	"github.com/boot2docker/boot2docker-cli/driver"
)

// Initialize the boot2docker VM from scratch.
func cmdInit() error {
	B2D.Init = true
	_, err := driver.GetMachine(&B2D)
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

	fmt.Printf("Waiting for VM to be started...")
	//give the VM a little time to start, so we don't kill the Serial Pipe/Socket
	time.Sleep(600 * time.Millisecond)
	natSSH := fmt.Sprintf("localhost:%d", m.GetSSHPort())
	IP := ""
	for i := 1; i < 30; i++ {
		if B2D.Serial && runtime.GOOS != "windows" {
			if IP = RequestIPFromSerialPort(m.GetSerialFile()); IP != "" {
				break
			}
		}
		if err := read(natSSH, 1, 2*time.Second); err == nil {
			IP = RequestIPFromSSH(m)
			break
		}

		print(".")
	}
	print("\n")

	fmt.Printf("Started.")

	if IP == "" {
		// lets try one more time
		time.Sleep(600 * time.Millisecond)
		fmt.Printf("  Trying to get IP one more time")

		IP = RequestIPFromSSH(m)
	}
	_ = RequestCertsUsingSSH(m)
	switch runtime.GOOS {
	case "windows":
		fmt.Printf("Docker client does not run on Windows for now. Please use")
		fmt.Printf("    \"%s\" ssh", os.Args[0])
		fmt.Printf("to SSH into the VM instead.")
	default:
		if IP == "" {
			fmt.Fprintf(os.Stderr, "Auto detection of the VM's IP address failed.")
			fmt.Fprintf(os.Stderr, "Please run `boot2docker -v up` to diagnose.")
		} else {
			// Check if $DOCKER_HOST ENV var is properly configured.
			socket := RequestSocketFromSSH(m)
			if os.Getenv("DOCKER_HOST") != socket {
				fmt.Printf("To connect the Docker client to the Docker daemon, please set:")
				fmt.Printf("    export DOCKER_HOST=%s", socket)
			} else {
				fmt.Printf("Your DOCKER_HOST env variable is already set correctly.")
			}
		}
	}
	return nil
}

// Tell the user the config (and later let them set it?)
func cmdConfig() error {
	dir, err := cfgDir(".boot2docker")
	if err != nil {
		return fmt.Errorf("Error working out Profile file location: %s", err)
	}
	filename := cfgFilename(dir)
	fmt.Printf("boot2docker profile filename: %s", filename)
	fmt.Println(printConfig())
	return nil
}

// Suspend and save the current state of VM on disk.
func cmdSave() error {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		return fmt.Errorf("Failed to get machine %q: %s", B2D.VM, err)
	}
	if err := m.Save(); err != nil {
		return fmt.Errorf("Failed to save machine %q: %s", B2D.VM, err)
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

	Socket := RequestSocketFromSSH(m)

	fmt.Fprintf(os.Stderr, "\n\t export DOCKER_HOST=")
	fmt.Printf("%s", Socket)
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
		for i := 1; i < 20; i++ {
			if runtime.GOOS != "windows" {
				if IP = RequestIPFromSerialPort(m.GetSerialFile()); IP != "" {
					break
				}
			}
		}
	}

	if IP == "" {
		IP = RequestIPFromSSH(m)
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
	fmt.Printf("Downloading boot2docker ISO image...")
	url := "https://api.github.com/repos/boot2docker/boot2docker/releases"
	tag, err := getLatestReleaseName(url)
	if err != nil {
		return fmt.Errorf("Failed to get latest release: %s", err)
	}
	fmt.Printf("Latest release is %s", tag)

	url = fmt.Sprintf("https://github.com/boot2docker/boot2docker/releases/download/%s/boot2docker.iso", tag)
	if err := download(B2D.ISO, url); err != nil {
		return fmt.Errorf("Failed to download ISO image: %s", err)
	}
	fmt.Printf("Success: downloaded %s\n\tto %s", url, B2D.ISO)
	return nil
}
