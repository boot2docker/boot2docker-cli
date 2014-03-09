package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

/*
VirtualBox Machine State Transition

A VirtualBox machine can be in one of the following states:

- poweroff: The VM is powered off and no previous running state saved.
- running: The VM is running.
- paused: The VM is paused, but its state is not saved to disk. If you quit
	      VirtualBox, the state will be lost.
- saved: The VM is powered off, and the previous state is saved on disk.
- aborted: The VM process crashed. This should happen very rarely.

VBoxManage supports the following transitions between states:

- startvm <VM>: poweroff|saved --> running
- controlvm <VM> pause: running --> paused
- controlvm <VM> resume: paused --> running
- controlvm <VM> savestate: running -> saved
- controlvm <VM> acpipowerbutton: running --> poweroff
- controlvm <VM> poweroff: running --> poweroff (unsafe)
- controlvm <VM> reset: running --> poweroff --> running (unsafe)

Poweroff and reset are unsafe because they will lose state and might corrupt
disk image.

To make things simpler, we do not expose the seldomly used paused state. We
define the following transitions instead:

- up|start: poweroff|saved|paused|aborted --> running
- down|halt|stop: [paused|saved -->] running --> poweroff
- save|suspend: [paused -->] running --> saved
- restart: [paused|saved -->] running --> poweroff --> running
- poweroff: [paused|saved -->] running --> poweroff (unsafe)
- reset: [paused|saved -->] running --> poweroff --> running (unsafe)

The takeaway is we try our best to transit the virtual machine into the state
you want it to be, and you only need to watch out for the potentially unsafe
poweroff and reset.
*/

// State of a virtual machine.
type vmState string

// VM state reported by VirtualBox. Note that `VBoxManage showvminfo` prints
// slightly different strings if supplied with `--machinereadable` flag. We
// use that flag as it's easier to parse.
const (
	vmPoweroff vmState = "poweroff"
	vmRunning          = "running"
	vmPaused           = "paused"
	vmSaved            = "saved"
	vmAborted          = "aborted"
)

// The following shadow states are not actually reported by VirtualBox. We
// invented them to make handling code simpler.
const (
	vmUnregistered vmState = "(unregistered)" // No such VM registerd.
	vmVBMNotFound          = "(VBMNotFound)"  // VBoxManage cannot be found.
	vmUnknown              = "(unknown)"      // Any other unknown state.
)

// Initialize the boot2docker VM from scratch.
func cmdInit() int {
	// TODO(@riobard) break up this command into multiple stages

	switch status(B2D.VM) {
	case vmUnregistered:
		break
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	default:
		logf("VM %q already exists.", B2D.VM)
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

	if _, err := os.Stat(B2D.ISO); err != nil {
		if !os.IsNotExist(err) {
			logf("Failed to open ISO image %q: %s", B2D.ISO, err)
			return 1
		}

		if exitcode := cmdDownload(); exitcode != 0 {
			return exitcode
		}
	}

	logf("Creating VM %s...", B2D.VM)
	if err := vbm("createvm", "--name", B2D.VM, "--register"); err != nil {
		logf("Failed to create VM %q: %s", B2D.VM, err)
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
		logf("Failed to modify VM %q: %s", B2D.VM, err)
		return 1
	}

	logf("Setting VM networking...")
	if err := vbm("modifyvm", B2D.VM,
		"--nic1", "nat",
		"--nictype1", "virtio",
		"--cableconnected1", "on",
	); err != nil {
		logf("Failed to add network interface to VM %q: %s", B2D.VM, err)
		return 1
	}

	if err := vbm("modifyvm", B2D.VM,
		"--natpf1", fmt.Sprintf("ssh,tcp,127.0.0.1,%d,,22", B2D.SSHPort),
		"--natpf1", fmt.Sprintf("docker,tcp,127.0.0.1,%d,,4243", B2D.DockerPort),
	); err != nil {
		logf("Failed to add port forwarding to VM %q: %s", B2D.VM, err)
		return 1
	}
	logf("Port forwarding [ssh]: host tcp://127.0.0.1:%d --> guest tcp://0.0.0.0:22", B2D.SSHPort)
	logf("Port forwarding [docker]: host tcp://127.0.0.1:%d --> guest tcp://0.0.0.0:4243", B2D.DockerPort)

	logf("Setting VM host-only networking")
	hostifname, err := getHostOnlyNetworkInterface()
	if err != nil {
		logf("Failed to create host-only network interface: %s", err)
		return 1
	}

	logf("Adding VM host-only networking interface %s", hostifname)
	if err := vbm("modifyvm", B2D.VM,
		"--nic2", "hostonly",
		"--nictype2", "virtio",
		"--cableconnected2", "on",
		"--hostonlyadapter2", hostifname,
	); err != nil {
		logf("Failed to add network interface to VM %q: %s", B2D.VM, err)
		return 1
	}

	logf("Setting VM storage...")
	if err := vbm("storagectl", B2D.VM,
		"--name", "SATA",
		"--add", "sata",
		"--hostiocache", "on",
	); err != nil {
		logf("Failed to add storage controller to VM %q: %s", B2D.VM, err)
		return 1
	}

	if err := vbm("storageattach", B2D.VM,
		"--storagectl", "SATA",
		"--port", "0",
		"--device", "0",
		"--type", "dvddrive",
		"--medium", B2D.ISO,
	); err != nil {
		logf("Failed to attach ISO image %q: %s", B2D.ISO, err)
		return 1
	}

	vmDir := basefolder(B2D.VM)
	diskImg := filepath.Join(vmDir, fmt.Sprintf("%s.vmdk", B2D.VM))

	if _, err := os.Stat(diskImg); err != nil {
		if !os.IsNotExist(err) {
			logf("Failed to open disk image %q: %s", diskImg, err)
			return 1
		}

		if err := makeDiskImage(diskImg, B2D.DiskSize); err != nil {
			logf("Failed to create disk image %q: %s", diskImg, err)
			return 1
		}
	}

	if err := vbm("storageattach", B2D.VM,
		"--storagectl", "SATA",
		"--port", "1",
		"--device", "0",
		"--type", "hdd",
		"--medium", diskImg,
	); err != nil {
		logf("Failed to attach disk image %q: %s", diskImg, err)
		return 1
	}

	logf("Done. Type `%s up` to start the VM.", os.Args[0])
	return 0
}

// Bring up the VM from all possible states.
func cmdUp() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q is not registered.", B2D.VM)
		return 1
	case vmRunning:
		logf("VM %q is already running.", B2D.VM)
	case vmPaused:
		logf("Resuming VM %q", B2D.VM)
		if err := vbm("controlvm", B2D.VM, "resume"); err != nil {
			logf("Failed to resume VM %q: %s", B2D.VM, err)
			return 1
		}
	case vmSaved, vmPoweroff, vmAborted:
		logf("Starting VM %q...", B2D.VM)
		if err := vbm("startvm", B2D.VM, "--type", "headless"); err != nil {
			logf("Failed to start VM %q: %s", B2D.VM, err)
			return 1
		}
	default:
		logf("Cannot start VM %q from state %s", B2D.VM, state)
		return 1
	}

	logf("Waiting for SSH server to start...")
	addr := fmt.Sprintf("localhost:%d", B2D.SSHPort)
	if err := read(addr); err != nil {
		logf("Failed to connect to SSH port at %s: %s", addr, err)
		return 1
	}
	logf("Started.")

	switch runtime.GOOS {
	case "windows":
		logf("Docker client does not run on Windows for now. Please use")
		logf("    %s ssh", os.Args[0])
		logf("to SSH into the VM instead.")
	default:
		// Check if $DOCKER_HOST ENV var is properly configured.
		if os.Getenv("DOCKER_HOST") != fmt.Sprintf("tcp://localhost:%d", B2D.DockerPort) {
			logf("To connect the Docker client to the Docker daemon, please set:")
			logf("    export DOCKER_HOST=tcp://localhost:%d", B2D.DockerPort)
		}
	}
	return 0
}

// Suspend and save the current state of VM on disk.
func cmdSave() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q is not registered.", B2D.VM)
		return 1
	case vmPaused: // resume from paused before saving
		if exitcode := cmdUp(); exitcode != 0 {
			return exitcode
		}
	case vmRunning:
		break
	default:
		logf("VM %q is not running.", B2D.VM)
		return 0
	}

	logf("Suspending VM %q", B2D.VM)
	if err := vbm("controlvm", B2D.VM, "savestate"); err != nil {
		logf("Failed to suspend VM %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Pause the VM.
func cmdPause() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q is not registered.", B2D.VM)
		return 1
	case vmRunning:
		break
	default:
		logf("VM %q is not running.", B2D.VM)
		return 0
	}

	if err := vbm("controlvm", B2D.VM, "pause"); err != nil {
		logf("Failed to pause VM %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Gracefully stop the VM by sending ACPI shutdown signal.
func cmdStop() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q is not registered.", B2D.VM)
		return 1
	case vmPaused, vmSaved: // resume before stopping
		if exitcode := cmdUp(); exitcode != 0 {
			return exitcode
		}
	case vmRunning:
		break
	default:
		logf("VM %q is not running.", B2D.VM)
		return 0
	}

	logf("Shutting down VM %q...", B2D.VM)
	if err := vbm("controlvm", B2D.VM, "acpipowerbutton"); err != nil {
		logf("Failed to shutdown VM %q: %s", B2D.VM, err)
		return 1
	}
	for status(B2D.VM) == vmRunning {
		time.Sleep(1 * time.Second)
	}
	return 0
}

// Forcefully power off the VM (equivalent to unplug power). Might corrupt disk
// image.
func cmdPoweroff() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q is not registered.", B2D.VM)
		return 1
	case vmRunning, vmPaused:
		break
	case vmSaved:
		if exitcode := cmdUp(); exitcode != 0 {
			return exitcode
		}
	default:
		logf("VM %q is not running.", B2D.VM)
		return 0
	}

	if err := vbm("controlvm", B2D.VM, "poweroff"); err != nil {
		logf("Failed to poweroff VM %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Gracefully stop and then start the VM.
func cmdRestart() int {
	switch state := status(B2D.VM); state {
	case vmPaused, vmSaved:
		if exitcode := cmdUp(); exitcode != 0 {
			return exitcode
		}
		fallthrough // important!
	case vmRunning:
		if exitcode := cmdStop(); exitcode != 0 {
			return exitcode
		}
	default:
		logf("Cannot restart VM %q from state %s", B2D.VM, state)
		return 1
	}
	return cmdUp()
}

// Forcefully reset (equivalent to cold boot) the VM. Might corrupt disk image.
func cmdReset() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q is not registered.", B2D.VM)
		return 1
	case vmPaused, vmSaved:
		if exitcode := cmdUp(); exitcode != 0 {
			return exitcode
		}
	case vmRunning:
		break
	default:
		logf("VM %q is not running.", B2D.VM)
		return 0
	}

	if err := vbm("controlvm", B2D.VM, "reset"); err != nil {
		logf("Failed to reset VM %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Delete the VM and associated disk image.
func cmdDelete() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q is not registered.", B2D.VM)
	case vmRunning, vmPaused:
		if exitcode := cmdPoweroff(); exitcode != 0 {
			return exitcode
		}
	}

	if err := vbm("unregistervm", "--delete", B2D.VM); err != nil {
		logf("Failed to delete VM %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Show detailed info of the VM.
func cmdInfo() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("%q does not exist", B2D.VM)
		return 1
	}

	if err := vbm("showvminfo", B2D.VM); err != nil {
		logf("Failed to show info of VM %q: %s", B2D.VM, err)
		return 1
	}
	return 0
}

// Show the current state of the VM.
func cmdStatus() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q does not exist", B2D.VM)
		return 1
	default:
		fmt.Println(state)
		return 0
	}
}

// Call the external SSH command to login into boot2docker VM.
func cmdSSH() int {
	switch state := status(B2D.VM); state {
	case vmVBMNotFound:
		logf("Failed to locate VirtualBox management utility %q", B2D.VBM)
		return 2
	case vmUnregistered:
		logf("VM %q is not registered.", B2D.VM)
		return 1
	case vmRunning:
		break
	default:
		logf("VM %q is not running.", B2D.VM)
		return 1
	}

	// TODO What SSH client is used on Windows? Does it support the options?
	if err := cmd(B2D.SSH,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", fmt.Sprintf("%d", B2D.SSHPort),
		"docker@localhost",
	); err != nil {
		logf("%s", err)
		return 1
	}
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
	return 0
}
