package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestExportCommandWritten(t *testing.T) {
	/*
		The export command is written to the output interface.
	*/
	var stdout bytes.Buffer
	m := GetDummyMachine()
	cmdShellSetup(m, &stdout)
	result := stdout.String()
	expected := "export DOCKER_HOST=tcp://192.0.2.1:1234\n"
	if result != expected {
		t.Error(fmt.Sprintf("Got %#v, expected %#v", result, expected))
	}
}
