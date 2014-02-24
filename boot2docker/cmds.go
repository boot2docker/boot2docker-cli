package main

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// State of a virtual machine.
type vmState string

const (
	vmRunning      vmState = "running"
	vmPoweroff             = "poweroff"
	vmPaused               = "paused"
	vmSaved                = "saved"
	vmAborted              = "aborted"
	vmUnregistered         = "(unregistered)" // not actually reported by VirtualBox
	vmUnknown              = "(unknown)"      // not actually reported by VirtualBox
)

// Call the external SSH command to login into boot2docker VM.
func cmdSSH() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)
		return 1
	case vmRunning:
		// TODO What SSH client is used on Windows?
		if err := cmd(B2D.SSH,
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-p", fmt.Sprintf("%d", B2D.SSHPort),
			"docker@localhost"); err != nil {
			logf("%s", err)
			return 1
		}
	default:
		logf("%s is not running.", B2D.VM)
		return 1
	}
	return 0
}

// Start the VM from all possible states.
func cmdStart() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)
		return 1
	case vmRunning:
		logf("%s is already running.", B2D.VM)
	case vmPaused:
		logf("Resuming %s", B2D.VM)
		if err := vbm("controlvm", B2D.VM, "resume"); err != nil {
			logf("Failed to resume vm: %s", err)
			return 1
		}
		addr := fmt.Sprintf("localhost:%d", B2D.SSHPort)
		if err := read(addr); err != nil {
			logf("Failed to connect to SSH port at %s: %s", addr, err)
			return 1
		}
		logf("Resumed.")
	case vmSaved, vmPoweroff, vmAborted:
		logf("Starting %s...", B2D.VM)
		if err := vbm("startvm", B2D.VM, "--type", "headless"); err != nil {
			logf("Failed to start vm: %s", err)
			return 1
		}
		logf("Waiting for SSH server to start...")
		addr := fmt.Sprintf("localhost:%d", B2D.SSHPort)
		if err := read(addr); err != nil {
			logf("Failed to connect to SSH port at %s: %s", addr, err)
			return 1
		}
		logf("Started.")
	default:
		logf("Cannot start %s from state %.", B2D.VM, state)
		return 1
	}

	// Check if $DOCKER_HOST ENV var is properly configured.
	DockerHost := getenv("DOCKER_HOST", "")
	if DockerHost != fmt.Sprintf("tcp://localhost:%d", B2D.DockerPort) {
		fmt.Printf("\nTo connect the docker client to the Docker daemon, please set:\n")
		fmt.Printf("export DOCKER_HOST=tcp://localhost:%d\n\n", B2D.DockerPort)
	}
	return 0
}

// Save the current state of VM on disk.
func cmdSave() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)
		return 1
	case vmRunning:
		logf("Suspending %s", B2D.VM)
		if err := vbm("controlvm", B2D.VM, "savestate"); err != nil {
			logf("Failed to suspend vm: %s", err)
			return 1
		}
	default:
		logf("%s is not running.", B2D.VM)
	}
	return 0
}

// Pause the VM.
func cmdPause() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)
		return 1
	case vmRunning:
		if err := vbm("controlvm", B2D.VM, "pause"); err != nil {
			logf("%s", err)
			return 1
		}
	default:
		logf("%s is not running.", B2D.VM)
	}
	return 0
}

// Gracefully stop the VM by sending ACPI shutdown signal.
func cmdStop() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)
		return 1
	case vmRunning:
		logf("Shutting down %s...", B2D.VM)
		if err := vbm("controlvm", B2D.VM, "acpipowerbutton"); err != nil {
			logf("Failed to shutdown vm: %s", err)
			return 1
		}
		for status(B2D.VM) == vmRunning {
			time.Sleep(1 * time.Second)
		}
	default:
		logf("%s is not running.", B2D.VM)
	}
	return 0
}

// Forcefully power off the VM (equivalent to unplug power). Could potentially
// result in corrupted disk. Use with care.
func cmdPoweroff() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)
		return 1
	case vmRunning:
		if err := vbm("controlvm", B2D.VM, "poweroff"); err != nil {
			logf("%s", err)
			return 1
		}
	default:
		logf("%s is not running.", B2D.VM)
	}
	return 0
}

// Gracefully stop and then start the VM.
func cmdRestart() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)
		return 1
	case vmRunning:
		if exitcode := cmdStop(); exitcode != 0 {
			return exitcode
		}
		fallthrough
	default:
		return cmdStart()
	}
	return 0
}

// Forcefully reset the VM. Could potentially result in corrupted disk. Use
// with care.
func cmdReset() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)
		return 1
	case vmRunning:
		if err := vbm("controlvm", B2D.VM, "reset"); err != nil {
			logf("%s", err)
			return 1
		}
	default:
		logf("%s is not running.", B2D.VM)
	}
	return 0
}

// Delete the VM and remove associated files.
func cmdDelete() int {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		logf("%s is not registered.", B2D.VM)

	case vmPoweroff, vmAborted:
		if err := vbm("unregistervm", "--delete", B2D.VM); err != nil {
			logf("Failed to delete vm: %s", err)
			return 1
		}
	default:
		logf("%s needs to be stopped to delete it.", B2D.VM)
		return 1
	}
	return 0
}

// Show detailed info of the VM.
func cmdInfo() int {
	if err := vbm("showvminfo", B2D.VM); err != nil {
		logf("%s", err)
		return 1
	}
	return 0
}

// Show the current state of the VM.
func cmdStatus() int {
	fmt.Printf("%s\n", status(B2D.VM))
	return 0
}

// Initialize the boot2docker VM from scratch.
func cmdInit() int {
	if state := status(B2D.VM); state != vmUnregistered {
		logf("%q already exists.", B2D.VM)
		return 1
	}

	if ping(fmt.Sprintf("localhost:%d", B2D.DockerPort)) {
		logf("DOCKER_PORT=%d on localhost is occupied. Please choose another none.", B2D.DockerPort)
		return 1
	}

	if ping(fmt.Sprintf("localhost:%d", B2D.SSHPort)) {
		logf("SSH_PORT=%d on localhost is occupied. Please choose another one.", B2D.SSHPort)
		return 1
	}

	logf("Creating VM %s...", B2D.VM)
	if err := vbm("createvm", "--name", B2D.VM, "--register"); err != nil {
		logf("Failed to create vm: %s", err)
		return 1
	}

	if err := vbm("modifyvm", B2D.VM,
		"--ostype", "Linux26_64",
		"--cpus", fmt.Sprintf("%d", runtime.NumCPU()),
		"--memory", fmt.Sprintf("%d", B2D.Memory),
		"--rtcuseutc", "on",
		"--acpi", "on",
		"--ioapic", "on",
		"--hpet", "on",
		"--hwvirtex", "on",
		"--vtxvpid", "on",
		"--largepages", "on",
		"--nestedpaging", "on",
		"--firmware", "bios",
		"--bioslogofadein", "off",
		"--bioslogofadeout", "off",
		"--bioslogodisplaytime", "0",
		"--biosbootmenu", "disabled",
		"--boot1", "dvd",
	); err != nil {
		logf("Failed to modify %s: %s", B2D.VM, err)
		return 1
	}

	logf("Setting VM networking...")
	if err := vbm("modifyvm", B2D.VM,
		"--nic1", "nat",
		"--nictype1", "virtio",
		"--cableconnected1", "on",
	); err != nil {
		logf("Failed to modify %s: %s", B2D.VM, err)
		return 1
	}

	if err := vbm("modifyvm", B2D.VM,
		"--natpf1", fmt.Sprintf("ssh,tcp,127.0.0.1,%d,,22", B2D.SSHPort),
		"--natpf1", fmt.Sprintf("docker,tcp,127.0.0.1,%d,,4243", B2D.DockerPort),
	); err != nil {
		logf("Failed to modify %s: %s", B2D.VM, err)
		return 1
	}
	logf("Port forwarding [ssh]: host tcp://127.0.0.1:%d --> guest tcp://0.0.0.0:22", B2D.SSHPort)
	logf("Port forwarding [docker]: host tcp://127.0.0.1:%d --> guest tcp://0.0.0.0:4243", B2D.DockerPort)

	if _, err := os.Stat(B2D.ISO); err != nil {
		if os.IsNotExist(err) {
			if exitcode := cmdDownload(); exitcode != 0 {
				return exitcode
			}
		} else {
			logf("Failed to open ISO image: %s", err)
			return 1
		}
	}

	if _, err := os.Stat(B2D.Disk); err != nil {
		if os.IsNotExist(err) {
			if err := makeDiskImage(B2D.Disk, B2D.DiskSize); err != nil {
				logf("Failed to create disk image: %s", err)
				return 1
			}
		} else {
			logf("Failed to open disk image: %s", err)
			return 1
		}
	}

	logf("Setting VM disks...")
	if err := vbm("storagectl", B2D.VM,
		"--name", "SATA",
		"--add", "sata",
		"--hostiocache", "on",
	); err != nil {
		logf("Failed to add storage controller: %s", err)
		return 1
	}

	if err := vbm("storageattach", B2D.VM,
		"--storagectl", "SATA",
		"--port", "0",
		"--device", "0",
		"--type", "dvddrive",
		"--medium", B2D.ISO,
	); err != nil {
		logf("Failed to attach storage device %s: %s", B2D.ISO, err)
		return 1
	}

	if err := vbm("storageattach", B2D.VM,
		"--storagectl", "SATA",
		"--port", "1",
		"--device", "0",
		"--type", "hdd",
		"--medium", B2D.Disk,
	); err != nil {
		logf("Failed to attach storage device %s: %s", B2D.Disk, err)
		return 1
	}

	logf("Done. Type `%s up` to start the VM.", os.Args[0])
	return 0
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
	logf("finished %s", B2D.ISO)
	return 0
}
