package main

import (
	vbx "github.com/boot2docker/boot2docker-cli/virtualbox"
)

// GetDummyMachine returns a dummy Machine object for tests
func GetDummyMachine() *vbx.Machine {
	m := &vbx.Machine{
		Name:       "dummy",
		UUID:       "dummy",
		DockerPort: 1234,
	}
	return m
}
