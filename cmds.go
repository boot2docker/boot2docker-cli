package main

import (
	"fmt"
	"log"
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
func cmdSSH() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Fatalf("%s is not registered.", B2D.VM)
	case vmRunning:
		if err := cmd(B2D.SSH,
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-p", fmt.Sprintf("%d", B2D.SSHPort),
			"docker@localhost"); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("%s is not running.", B2D.VM)
	}
}

// Start the VM from all possible states.
func cmdStart() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Fatalf("%s is not registered.", B2D.VM)
	case vmRunning:
		log.Printf("%s is already running.", B2D.VM)
	case vmPaused:
		log.Printf("Resuming %s", B2D.VM)
		if err := vbm("controlvm", B2D.VM, "resume"); err != nil {
			log.Fatalf("Failed to resume vm: %s", err)
		}
		addr := fmt.Sprintf("localhost:%d", B2D.SSHPort)
		if err := read(addr); err != nil {
			log.Fatalf("Failed to connect to SSH port at %s: %s", addr, err)
		}
		log.Printf("Resumed.")
	case vmSaved, vmPoweroff, vmAborted:
		log.Printf("Starting %s...", B2D.VM)
		if err := vbm("startvm", B2D.VM, "--type", "headless"); err != nil {
			log.Fatalf("Failed to start vm: %s", err)
		}
		log.Printf("Waiting for SSH server to start...")
		addr := fmt.Sprintf("localhost:%d", B2D.SSHPort)
		if err := read(addr); err != nil {
			log.Fatalf("Failed to connect to SSH port at %s: %s", addr, err)
		}
		log.Printf("Started.")
	default:
		log.Fatalf("Cannot start %s from state %.", B2D.VM, state)
	}

	// Check if $DOCKER_HOST ENV var is properly configured.
	DockerHost := getenv("DOCKER_HOST", "")
	if DockerHost != fmt.Sprintf("tcp://localhost:%d", B2D.DockerPort) {
		fmt.Printf("\nTo connect the docker client to the Docker daemon, please set:\n")
		fmt.Printf("export DOCKER_HOST=tcp://localhost:%d\n\n", B2D.DockerPort)
	}
}

// Save the current state of VM on disk.
func cmdSave() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Fatalf("%s is not registered.", B2D.VM)
	case vmRunning:
		log.Printf("Suspending %s", B2D.VM)
		if err := vbm("controlvm", B2D.VM, "savestate"); err != nil {
			log.Fatalf("Failed to suspend vm: %s", err)
		}
	default:
		log.Printf("%s is not running.", B2D.VM)
	}
}

// Pause the VM.
func cmdPause() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Fatalf("%s is not registered.", B2D.VM)
	case vmRunning:
		if err := vbm("controlvm", B2D.VM, "pause"); err != nil {
			log.Fatal(err)
		}
	default:
		log.Printf("%s is not running.", B2D.VM)
	}
}

// Gracefully stop the VM by sending ACPI shutdown signal.
func cmdStop() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Fatalf("%s is not registered.", B2D.VM)
	case vmRunning:
		log.Printf("Shutting down %s...", B2D.VM)
		if err := vbm("controlvm", B2D.VM, "acpipowerbutton"); err != nil {
			log.Fatalf("Failed to shutdown vm: %s", err)
		}
		for status(B2D.VM) == vmRunning {
			time.Sleep(1 * time.Second)
		}
	default:
		log.Printf("%s is not running.", B2D.VM)
	}
}

// Forcefully power off the VM (equivalent to unplug power). Could potentially
// result in corrupted disk. Use with care.
func cmdPoweroff() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Fatalf("%s is not registered.", B2D.VM)
	case vmRunning:
		if err := vbm("controlvm", B2D.VM, "poweroff"); err != nil {
			log.Fatal(err)
		}
	default:
		log.Printf("%s is not running.", B2D.VM)
	}
}

// Gracefully stop and then start the VM.
func cmdRestart() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Fatalf("%s is not registered.", B2D.VM)
	case vmRunning:
		cmdStop()
		time.Sleep(1 * time.Second)
		cmdStart()
	default:
		cmdStart()
	}
}

// Forcefully reset the VM. Could potentially result in corrupted disk. Use
// with care.
func cmdReset() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Fatalf("%s is not registered.", B2D.VM)
	case vmRunning:
		if err := vbm("controlvm", B2D.VM, "reset"); err != nil {
			log.Fatal(err)
		}
	default:
		log.Printf("%s is not running.", B2D.VM)
	}
}

// Delete the VM and remove associated files.
func cmdDelete() {
	switch state := status(B2D.VM); state {
	case vmUnregistered:
		log.Printf("%s is not registered.", B2D.VM)

	case vmPoweroff, vmAborted:
		if err := vbm("unregistervm", "--delete", B2D.VM); err != nil {
			log.Fatalf("Failed to delete vm: %s", err)
		}
	default:
		log.Fatalf("%s needs to be stopped to delete it.", B2D.VM)
	}
}

// Show detailed info of the VM.
func cmdInfo() {
	if err := vbm("showvminfo", B2D.VM); err != nil {
		log.Fatal(err)
	}
}

// Show the current state of the VM.
func cmdStatus() {
	state := status(B2D.VM)
	fmt.Printf("%s is %s.\n", B2D.VM, state)
	if state != vmRunning {
		os.Exit(1)
	}
}

// Initialize the boot2docker VM from scratch.
func cmdInit() {
	if state := status(B2D.VM); state != vmUnregistered {
		log.Fatalf("%s already exists.\n")
	}

	if ping(fmt.Sprintf("localhost:%d", B2D.DockerPort)) {
		log.Fatalf("DOCKER_PORT=%d on localhost is occupied. Please choose another none.", B2D.DockerPort)
	}

	if ping(fmt.Sprintf("localhost:%d", B2D.SSHPort)) {
		log.Fatalf("SSH_HOST_PORT=%d on localhost is occupied. Please choose another one.", B2D.SSHPort)
	}

	log.Printf("Creating VM %s...", B2D.VM)
	if err := vbm("createvm", "--name", B2D.VM, "--register"); err != nil {
		log.Fatalf("Failed to create vm: %s", err)
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
		"--boot1", "dvd"); err != nil {
		log.Fatal("Failed to modify vm: %s", err)
	}

	log.Printf("Setting VM networking")
	if err := vbm("modifyvm", B2D.VM, "--nic1", "nat", "--nictype1", "virtio", "--cableconnected1", "on"); err != nil {
		log.Fatalf("Failed to modify vm: %s", err)
	}

	if err := vbm("modifyvm", B2D.VM,
		"--natpf1", fmt.Sprintf("ssh,tcp,127.0.0.1,%d,,22", B2D.SSHPort),
		"--natpf1", fmt.Sprintf("docker,tcp,127.0.0.1,%d,,4243", B2D.DockerPort)); err != nil {
		log.Fatalf("Failed to modify vm: %s", err)
	}
	log.Printf("Port forwarding [ssh]: host tcp://127.0.0.1:%d --> guest tcp://0.0.0.0:22", B2D.SSHPort)
	log.Printf("Port forwarding [docker]: host tcp://127.0.0.1:%d --> guest tcp://0.0.0.0:4243", B2D.DockerPort)

	if _, err := os.Stat(B2D.ISO); err != nil {
		if os.IsNotExist(err) {
			cmdDownload()
		} else {
			log.Fatalf("Failed to open ISO image: %s", err)
		}
	}

	if _, err := os.Stat(B2D.Disk); err != nil {
		if os.IsNotExist(err) {
			err := makeDiskImage(B2D.Disk, B2D.DiskSize)
			if err != nil {
				log.Fatalf("Failed to create disk image: %s", err)
			}
		} else {
			log.Fatalf("Failed to open disk image: %s", err)
		}
	}

	log.Printf("Setting VM disks")
	if err := vbm("storagectl", B2D.VM, "--name", "SATA", "--add", "sata", "--hostiocache", "on"); err != nil {
		log.Fatalf("Failed to add storage controller: %s", err)
	}

	if err := vbm("storageattach", B2D.VM, "--storagectl", "SATA", "--port", "0", "--device", "0", "--type", "dvddrive", "--medium", B2D.ISO); err != nil {
		log.Fatalf("Failed to attach storage device: %s", err)
	}

	if err := vbm("storageattach", B2D.VM, "--storagectl", "SATA", "--port", "1", "--device", "0", "--type", "hdd", "--medium", B2D.Disk); err != nil {
		log.Fatalf("Failed to attach storage device: %s", err)
	}

	log.Printf("Done.")
	log.Printf("You can now type `%s up` and wait for the VM to start.", os.Args[0])
}

// Download the boot2docker ISO image.
func cmdDownload() {
	log.Printf("downloading boot2docker ISO image...")
	tag, err := getLatestReleaseName("https://api.github.com/repos/boot2docker/boot2docker/releases")
	if err != nil {
		log.Fatalf("Failed to get latest release: %s", err)
	}
	log.Printf("  %s", tag)

	if err := download(B2D.ISO, fmt.Sprintf("https://github.com/boot2docker/boot2docker/releases/download/%s/boot2docker.iso", tag)); err != nil {
		log.Fatalf("Failed to download ISO image: %s", err)
	}
}
