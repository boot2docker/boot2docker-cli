package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	_ "github.com/boot2docker/boot2docker-cli/dummy"
	_ "github.com/boot2docker/boot2docker-cli/virtualbox"

	"github.com/boot2docker/boot2docker-cli/driver"
)

// Initialize the boot2docker VM from scratch.
func cmdInit() int {
	B2D.Init = true
	_, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to initialize machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Bring up the VM from all possible states.
func cmdUp() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Start(); err != nil {
		logf("Failed to start machine %q: %s", B2D.VM, err)
		return 1
	}

	if err := m.Refresh(); err != nil {
		logf("Failed to start machine %q: %s", B2D.VM, err)
		return 1
	}
	if m.GetState() != driver.Running {
		logf("Failed to start machine %q (run again with -v for details)", B2D.VM)
		return 1
	}

	logf("Waiting for VM to be started...")
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

	logf("Started.")

	if IP == "" {
		// lets try one more time
		time.Sleep(600 * time.Millisecond)
		logf("  Trying to get IP one more time")

		IP = RequestIPFromSSH(m)
	}
	switch runtime.GOOS {
	case "windows":
		logf("Docker client does not run on Windows for now. Please use")
		logf("    \"%s\" ssh", os.Args[0])
		logf("to SSH into the VM instead.")
	default:
		if IP == "" {
			logf("Auto detection of the VM's IP address failed.")
			logf("Please run `boot2docker -v up` to diagnose.")
		} else {
			// Check if $DOCKER_HOST ENV var is properly configured.
			if os.Getenv("DOCKER_HOST") != fmt.Sprintf("tcp://%s:%d", IP, driver.DockerPort) {
				logf("To connect the Docker client to the Docker daemon, please set:")
				logf("    export DOCKER_HOST=tcp://%s:%d", IP, driver.DockerPort)
			} else {
				logf("Your DOCKER_HOST env variable is already set correctly.")
			}
		}
	}
	return 0
}

// Tell the user the config (and later let them set it?)
func cmdConfig() int {
	dir, err := getCfgDir(".boot2docker")
	if err != nil {
		logf("Error working out Profile file location: %s", err)
		return 1
	}
	filename := getCfgFilename(dir)
	logf("boot2docker profile filename: %s", filename)
	fmt.Println(printConfig())
	return 0
}

// Suspend and save the current state of VM on disk.
func cmdSave() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Save(); err != nil {
		logf("Failed to save machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Gracefully stop the VM by sending ACPI shutdown signal.
func cmdStop() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Stop(); err != nil {
		logf("Failed to stop machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Forcefully power off the VM (equivalent to unplug power). Might corrupt disk
// image.
func cmdPoweroff() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Poweroff(); err != nil {
		logf("Failed to poweroff machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Upgrade the boot2docker iso - preserving server state
func cmdUpgrade() int {
	m, err := driver.GetMachine(&B2D)
	if err == nil && m.GetState() == driver.Running {
		// Windows won't let us move the iso aside while its in use
		if cmdStop() == 0 && cmdDownload() == 0 {
			return cmdUp()
		} else {
			return 0
		}
	} else {
		return cmdDownload()
	}
}

// Gracefully stop and then start the VM.
func cmdRestart() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Restart(); err != nil {
		logf("Failed to restart machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Forcefully reset (equivalent to cold boot) the VM. Might corrupt disk image.
func cmdReset() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Reset(); err != nil {
		logf("Failed to reset machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Delete the VM and associated disk image.
func cmdDelete() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		if err == driver.ErrMachineNotExist {
			logf("Machine %q does not exist.", B2D.VM)
			return 0
		}
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := m.Delete(); err != nil {
		logf("Failed to delete machine %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Show detailed info of the VM.
func cmdInfo() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	if err := json.NewEncoder(os.Stdout).Encode(m); err != nil {
		logf("Failed to encode machine %q info: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Show the current state of the VM.
func cmdStatus() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}
	fmt.Println(m.GetState())
	return 0
}

// Call the external SSH command to login into boot2docker VM.
func cmdSSH() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}

	if m.GetState() != driver.Running {
		logf("VM %q is not running.", B2D.VM)
		return 1
	}

	// find the ssh cmd string and then pass any remaining strings to ssh
	// TODO: it's a shame to repeat the same code as in config.go, but I
	//       didn't find a way to share the unsharable without more rework
	i := 1
	for i < len(os.Args) && os.Args[i-1] != "ssh" {
		i++
	}

	sshArgs := append([]string{
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=quiet", // suppress "Warning: Permanently added '[localhost]:2022' (ECDSA) to the list of known hosts."
		"-p", fmt.Sprintf("%d", m.GetSSHPort()),
		"-i", B2D.SSHKey,
		"docker@localhost",
	}, os.Args[i:]...)

	if err := cmdInteractive(B2D.SSH, sshArgs...); err != nil {
		logf("%s", err)
		return 1
	}
	return 0
}

func cmdIP() int {
	m, err := driver.GetMachine(&B2D)
	if err != nil {
		logf("Failed to get machine %q: %s", B2D.VM, err)
		return 2
	}

	if m.GetState() != driver.Running {
		logf("VM %q is not running.", B2D.VM)
		return 1
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
		errf("\nThe VM's Host only interface IP address is: ")
		fmt.Printf("%s", IP)
		errf("\n\n")
	} else {
		errf("\nFailed to get VM Host only IP address.\n")
		errf("\tWas the VM initilized using boot2docker?\n")
	}
	return 0
}

func RequestIPFromSSH(m driver.Machine) string {
	// fall back to using the NAT port forwarded ssh
	out, err := cmd(B2D.SSH,
		"-v", // please leave in - this seems to improve the chance of success
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", fmt.Sprintf("%d", m.GetSSHPort()),
		"-i", B2D.SSHKey,
		"docker@localhost",
		"ip addr show dev eth1",
	)
	IP := ""
	if err != nil {
		logf("%s", err)
	} else {
		if B2D.Verbose {
			logf("SSH returned: %s\nEND SSH\n", out)
		}
		// parse to find: inet 192.168.59.103/24 brd 192.168.59.255 scope global eth1
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			vals := strings.Split(strings.TrimSpace(line), " ")
			if len(vals) >= 2 && vals[0] == "inet" {
				IP = vals[1][:strings.Index(vals[1], "/")]
				break
			}
		}
	}
	return IP
}

// Download the boot2docker ISO image.
func cmdDownload() int {
	logf("Downloading boot2docker ISO image...")
	url := "https://api.github.com/repos/boot2docker/boot2docker/releases"
	tag, err := getLatestReleaseName(url)
	if err != nil {
		logf("Failed to get latest release: %s", err)
		return 1
	}
	logf("Latest release is %s", tag)

	url = fmt.Sprintf("https://github.com/boot2docker/boot2docker/releases/download/%s/boot2docker.iso", tag)
	if err := download(B2D.ISO, url); err != nil {
		logf("Failed to download ISO image: %s", err)
		return 1
	}
	logf("Success: downloaded %s\n\tto %s", url, B2D.ISO)
	return 0
}
