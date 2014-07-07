package main

import (
    "testing"
	vbx "github.com/boot2docker/boot2docker-cli/virtualbox"
)

func TestInfo(t *testing.T) {
	config()
	m, err := vbx.GetMachine(B2D.VM)
    if err != nil {
        t.Errorf("%v", err)
    }
    if m.Name != "boot2docker-vm" {
        t.Error("Incorrect VM name")
    }
}
