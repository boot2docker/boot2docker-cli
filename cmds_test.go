package main

import (
    "bytes"
    "testing"
)

func TestExportCommandWritten(t* testing.T) {
    /*
    The export command is written to the output interface.
    */
    var stdout bytes.Buffer
    m  := GetDummyMachine()
    cmdShellSetup(m, &stdout)
    result := stdout.String()
    expected := "export DOCKER_HOST=tcp://alpha:1234"
    if result != expected {
        t.Error("Got", result, "expected", expected)
    }
}

