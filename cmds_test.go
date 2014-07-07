package main

import (
    "testing"
)

func TestDummyMachineIP(t *testing.T) {
    /*
    The IP address of a dummy machine is always "alpha".
    */
	m := GetDummyMachine()
    IP := GetIPForMachine(m)
    if IP != "alpha" {
        t.Fail()
    }
}
