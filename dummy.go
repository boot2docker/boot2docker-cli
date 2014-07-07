package main

import (
	vbx "github.com/boot2docker/boot2docker-cli/virtualbox"
)

// GetDummyMachine returns a dummy Machine object for tests
func GetDummyMachine() (*vbx.Machine) {
    m := &vbx.Machine{}
    m.Name = "dummy"
    m.UUID = "dummy"
    return m
}
