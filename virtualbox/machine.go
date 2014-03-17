package virtualbox

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
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

To make things simpler, the following transitions are used instead:

- start: poweroff|saved|paused|aborted --> running
- stop: [paused|saved -->] running --> poweroff
- save: [paused -->] running --> saved
- restart: [paused|saved -->] running --> poweroff --> running
- poweroff: [paused|saved -->] running --> poweroff (unsafe)
- reset: [paused|saved -->] running --> poweroff --> running (unsafe)

The takeaway is we try our best to transit the virtual machine into the state
you want it to be, and you only need to watch out for the potentially unsafe
poweroff and reset.
*/

type MachineState string

const (
	Poweroff MachineState = "poweroff"
	Running               = "running"
	Paused                = "paused"
	Saved                 = "saved"
	Aborted               = "aborted"
)

type Flag int

// Flag names in lowercases to be consistent with VBoxManage options.
const (
	F_acpi Flag = 1 << iota
	F_ioapic
	F_rtcuseutc
	F_cpuhotplug
	F_pae
	F_longmode
	F_synthcpu
	F_hpet
	F_hwvirtex
	F_triplefaultreset
	F_nestedpaging
	F_largepages
	F_vtxvpid
	F_vtxux
	F_accelerate3d
)

// Convert bool to "on"/"off"
func bool2string(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// Test if flag is set. Return "on" or "off".
func (f Flag) Get(o Flag) string {
	return bool2string(f&o == o)
}

// Machine information.
type Machine struct {
	Name       string
	UUID       string
	State      MachineState
	CPUs       uint
	Memory     uint // main memory (in MB)
	VRAM       uint // video memory (in MB)
	CfgFile    string
	BaseFolder string
	OSType     string
	Flag       Flag
	BootOrder  []string // max 4 slots, each in {none|floppy|dvd|disk|net}
}

// Refresh the machine information.
func (m *Machine) Refresh() error {
	id := m.Name
	if id == "" {
		id = m.UUID
	}
	mm, err := GetMachine(id)
	if err != nil {
		return err
	}
	*m = *mm
	return nil
}

// Start the machine.
func (m *Machine) Start() error {
	switch m.State {
	case Paused:
		return vbm("controlvm", m.Name, "resume")
	case Poweroff, Saved, Aborted:
		return vbm("startvm", m.Name, "--type", "headless")
	}
	return nil
}

// Suspend the machine and save its state to disk.
func (m *Machine) Save() error {
	switch m.State {
	case Paused:
		if err := m.Start(); err != nil {
			return err
		}
	case Poweroff, Aborted, Saved:
		return nil
	}
	return vbm("controlvm", m.Name, "savestate")
}

// Pause the execution of the machine.
func (m *Machine) Pause() error {
	switch m.State {
	case Paused, Poweroff, Aborted, Saved:
		return nil
	}
	return vbm("controlvm", m.Name, "pause")
}

// Gracefully stop the machine.
func (m *Machine) Stop() error {
	switch m.State {
	case Poweroff, Aborted, Saved:
		return nil
	case Paused:
		if err := m.Start(); err != nil {
			return err
		}
	}

	for m.State != Poweroff { // busy wait until the machine is stopped
		if err := vbm("controlvm", m.Name, "acpipowerbutton"); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		if err := m.Refresh(); err != nil {
			return err
		}
	}
	return nil
}

// Forcefully stop the machine. State is lost and might corrupt disk.
func (m *Machine) Poweroff() error {
	switch m.State {
	case Poweroff, Aborted, Saved:
		return nil
	}
	return vbm("controlvm", m.Name, "poweroff")
}

// Gracefully restart the machine.
func (m *Machine) Restart() error {
	switch m.State {
	case Paused, Saved:
		if err := m.Start(); err != nil {
			return err
		}
	}
	if err := m.Stop(); err != nil {
		return err
	}
	return m.Start()
}

// Forcefully restart the machine. State is lost and might corrupt disk.
func (m *Machine) Reset() error {
	switch m.State {
	case Paused, Saved:
		if err := m.Start(); err != nil {
			return err
		}
	}
	return vbm("controlvm", m.Name, "reset")
}

// Delete the machine and associated disk images.
func (m *Machine) Delete() error {
	if err := m.Poweroff(); err != nil {
		return err
	}
	return vbm("unregistervm", m.Name, "--delete")
}

// Get a machine by name or by UUID.
func GetMachine(id string) (*Machine, error) {
	stdout, stderr, err := vbmOutErr("showvminfo", id, "--machinereadable")
	if err != nil {
		if reMachineNotFound.Find(stderr) != nil {
			return nil, ErrMachineNotExist
		}
		return nil, err
	}
	s := bufio.NewScanner(bytes.NewReader(stdout))
	m := &Machine{}
	for s.Scan() {
		res := reVMInfoLine.FindStringSubmatch(s.Text())
		if res == nil {
			continue
		}
		key := res[1]
		if key == "" {
			key = res[2]
		}
		val := res[3]
		if val == "" {
			val = res[4]
		}

		switch key {
		case "name":
			m.Name = val
		case "UUID":
			m.UUID = val
		case "VMState":
			m.State = MachineState(val)
		case "memory":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, err
			}
			m.Memory = uint(n)
		case "cpus":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, err
			}
			m.CPUs = uint(n)
		case "vram":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return nil, err
			}
			m.VRAM = uint(n)
		case "CfgFile":
			m.CfgFile = val
			m.BaseFolder = filepath.Dir(val)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

// List all machines.
func ListMachines() ([]*Machine, error) {
	b, err := vbmOut("list", "vms")
	if err != nil {
		return nil, err
	}
	var ms []*Machine
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		res := reVMNameUUID.FindStringSubmatch(s.Text())
		if res == nil {
			continue
		}
		m, err := GetMachine(res[1])
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return ms, nil
}

// Create a new machine. If basefolder is empty, use default.
func CreateMachine(name, basefolder string) (*Machine, error) {
	if name == "" {
		return nil, fmt.Errorf("machine name is empty")
	}

	// Check if a machine with the given name already exists.
	ms, err := ListMachines()
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		if m.Name == name {
			return nil, ErrMachineExist
		}
	}

	// Create and register the machine.
	args := []string{"createvm", "--name", name, "--register"}
	if basefolder != "" {
		args = append(args, "--basefolder", basefolder)
	}
	if err := vbm(args...); err != nil {
		return nil, err
	}

	m, err := GetMachine(name)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Modify the settings of the machine.
func (m *Machine) Modify() error {
	args := []string{"modifyvm", m.Name,
		"--firmware", "bios",
		"--bioslogofadein", "off",
		"--bioslogofadeout", "off",
		"--bioslogodisplaytime", "0",
		"--biosbootmenu", "disabled",

		"--ostype", m.OSType,
		"--cpus", fmt.Sprintf("%d", m.CPUs),
		"--memory", fmt.Sprintf("%d", m.Memory),
		"--vram", fmt.Sprintf("%d", m.VRAM),

		"--acpi", m.Flag.Get(F_acpi),
		"--ioapic", m.Flag.Get(F_ioapic),
		"--rtcuseutc", m.Flag.Get(F_rtcuseutc),
		"--cpuhotplug", m.Flag.Get(F_cpuhotplug),
		"--pae", m.Flag.Get(F_pae),
		"--longmode", m.Flag.Get(F_longmode),
		"--synthcpu", m.Flag.Get(F_synthcpu),
		"--hpet", m.Flag.Get(F_hpet),
		"--hwvirtex", m.Flag.Get(F_hwvirtex),
		"--triplefaultreset", m.Flag.Get(F_triplefaultreset),
		"--nestedpaging", m.Flag.Get(F_nestedpaging),
		"--largepages", m.Flag.Get(F_largepages),
		"--vtxvpid", m.Flag.Get(F_vtxvpid),
		"--vtxux", m.Flag.Get(F_vtxux),
		"--accelerate3d", m.Flag.Get(F_accelerate3d),
	}

	for i, dev := range m.BootOrder {
		if i > 3 {
			break
		}
		args = append(args, fmt.Sprintf("--boot%d", i+1), dev)
	}
	if err := vbm(args...); err != nil {
		return err
	}
	return m.Refresh()
}

// Add a named NAT port forarding rule to NIC number #n.
func (m *Machine) AddNATPF(n int, name string, rule PFRule) error {
	return vbm("controlvm", m.Name, fmt.Sprintf("natpf%d", n),
		fmt.Sprintf("%s,%s", name, rule.Format()))
}

// Delete the named NAT port forwarding rule from NIC number #n.
func (m *Machine) DelNATPF(n int, name string) error {
	return vbm("controlvm", m.Name, fmt.Sprintf("natpf%d", n), "delete", name)
}

// Set the NIC number #n.
func (m *Machine) SetNIC(n int, nic NIC) error {
	args := []string{"modifyvm", m.Name,
		fmt.Sprintf("--nic%d", n), string(nic.Network),
		fmt.Sprintf("--nictype%d", n), string(nic.Hardware),
		fmt.Sprintf("--cableconnected%d", n), "on",
	}

	if nic.Network == "hostonly" {
		args = append(args, fmt.Sprintf("--hostonlyadapter%d", n), nic.HostonlyAdapter)
	}
	return vbm(args...)
}

// Add a named storage controller.
func (m *Machine) AddStorageCtl(name string, ctl StorageController) error {
	args := []string{"storagectl", m.Name, "--name", name}
	if ctl.SysBus != "" {
		args = append(args, "--add", string(ctl.SysBus))
	}
	if ctl.Ports > 0 {
		args = append(args, "--portcount", fmt.Sprintf("%d", ctl.Ports))
	}
	if ctl.Chipset != "" {
		args = append(args, "--controller", string(ctl.Chipset))
	}
	args = append(args, "--hostiocache", bool2string(ctl.HostIOCache))
	args = append(args, "--bootable", bool2string(ctl.Bootable))
	return vbm(args...)
}

// Delete the naed storage controller.
func (m *Machine) DelStorageCtl(name string) error {
	return vbm("storagectl", m.Name, "--name", name, "--remove")
}

// Attach a storage medium to the named storage controller.
func (m *Machine) AttachStorage(ctlName string, medium StorageMedium) error {
	return vbm("storageattach", m.Name, "--storagectl", ctlName,
		"--port", fmt.Sprintf("%d", medium.Port),
		"--device", fmt.Sprintf("%d", medium.Device),
		"--type", string(medium.DriveType),
		"--medium", medium.Medium,
	)
}
