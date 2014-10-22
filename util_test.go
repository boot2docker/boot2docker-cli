package main

import (
	"archive/tar"
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/boot2docker/boot2docker-cli/driver"
	"github.com/boot2docker/boot2docker-cli/dummy"
)

type FixedOutputer struct {
	output []byte
}

func (f FixedOutputer) Output() ([]byte, error) {
	return f.output, nil
}

func NewFixedOutputerStr(o string) FixedOutputer {
	return FixedOutputer{[]byte(o)}
}

func NewFixedOutputerByte(b []byte) FixedOutputer {
	return FixedOutputer{b}
}

type FakeSshCommander struct {
	builtIns map[string]Outputer
}

func NewFakeSshCommander() FakeSshCommander {
	f := FakeSshCommander{
		builtIns: map[string]Outputer{},
	}
	return f
}

func (f FakeSshCommander) AddCmdOut(cmd string, out string) {
	f.builtIns[cmd] = NewFixedOutputerStr(out)
}

func (f FakeSshCommander) AddTarCmd() {
	f.builtIns[SSHCommTarPems] = NewFixedOutputerByte(getTarBtes())
}

func getTarBtes() []byte {
	buf := new(bytes.Buffer)

	tw := tar.NewWriter(buf)

	body := "delme now"
	hdr := &tar.Header{
		Name: "delme.pem",
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		log.Fatalln(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		log.Fatalln(err)
	}

	if err := tw.Close(); err != nil {
		log.Fatalln(err)
	}
	return buf.Bytes()
}

func (f FakeSshCommander) GetSshCommand(m driver.Machine, args ...string) Outputer {
	a := strings.Join(args, " ")
	o := f.builtIns[a]
	return o
}

func TestRequestIPFromSSHFull(t *testing.T) {
	fc := NewFakeSshCommander()
	fc.AddCmdOut(SSHCommGetIp, `4: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
  link/ether 08:00:27:8f:93:ba brd ff:ff:ff:ff:ff:ff
  inet 111.222.111.222/24 brd 192.168.59.255 scope global eth1
     valid_lft forever preferred_lft forever
  inet6 fe80::a00:27ff:fe8f:93ba/64 scope link
     valid_lft forever preferred_lft forever`)
	sshProvider = fc.GetSshCommand

	ip, err := RequestIPFromSSH(nil)
	if err != nil {
		t.Fatal(err)
	}

	if ip != "111.222.111.222" {
		t.Fatalf("Expected 111.222.111.222 got %s", ip)
	}

}

func TestRequestSocketFromSSHZero(t *testing.T) {
	fc := NewFakeSshCommander()

	fc.AddCmdOut(SSHCommGetTcp, `tcp://0.0.0.0:2375`)
	fc.AddCmdOut(SSHCommGetIp, `4: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
    link/ether 08:00:27:8f:93:ba brd ff:ff:ff:ff:ff:ff
    inet 192.168.59.103/24 brd 192.168.59.255 scope global eth1
       valid_lft forever preferred_lft forever
    inet6 fe80::a00:27ff:fe8f:93ba/64 scope link
       valid_lft forever preferred_lft forever`)

	fc.AddCmdOut(SSHCommDaemonArgs, `/usr/local/bin/docker -d -D -g /var/lib/docker -H unix:// -H tcp://0.0.0.0:2375 -b=bridge0 --registry-mirror=http://192.168.1.111:5000`)
	sshProvider = fc.GetSshCommand

	s, err := RequestSocketFromSSH(nil)
	if err != nil {
		t.Fatal(err)
	}

	if s != "tcp://192.168.59.103:2375" {
		t.Fatalf("Expected tcp://192.168.59.103:2375 got %s", s)
	}

}

func TestRequestSocketFromSSH(t *testing.T) {
	fc := NewFakeSshCommander()

	fc.AddCmdOut(SSHCommGetTcp, `tcp://1.2.3.4:2375`)
	fc.AddCmdOut(SSHCommGetIp, `4: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
    link/ether 08:00:27:8f:93:ba brd ff:ff:ff:ff:ff:ff
    inet 192.168.59.103/24 brd 192.168.59.255 scope global eth1
       valid_lft forever preferred_lft forever
    inet6 fe80::a00:27ff:fe8f:93ba/64 scope link
       valid_lft forever preferred_lft forever`)

	fc.AddCmdOut(SSHCommDaemonArgs, `/usr/local/bin/docker --tlsverify -d -D -g /var/lib/docker -H unix:// -H tcp://0.0.0.0:2375 -b=bridge0 --registry-mirror=http://192.168.1.111:5000`)
	sshProvider = fc.GetSshCommand

	s, err := RequestSocketFromSSH(nil)
	if err != nil {
		t.Fatal(err)
	}

	if s != "tcp://1.2.3.4:2375" {
		t.Fatalf("Expected tcp://1.2.3.4:2375 got %s", s)
	}
}

func getDummyMachine() driver.Machine {
	c := driver.MachineConfig{
		VM:      "dummy",
		SSHPort: 22,
	}
	m, _ := dummy.InitFunc(&c)
	return m
}

func TestRequestCertsUsingSSH(t *testing.T) {
	fc := NewFakeSshCommander()

	fc.AddCmdOut(SSHCommGetTcp, `tcp://1.2.3.4:2375`)
	fc.AddCmdOut(SSHCommGetIp, `4: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
    link/ether 08:00:27:8f:93:ba brd ff:ff:ff:ff:ff:ff
    inet 192.168.59.103/24 brd 192.168.59.255 scope global eth1
       valid_lft forever preferred_lft forever
    inet6 fe80::a00:27ff:fe8f:93ba/64 scope link
       valid_lft forever preferred_lft forever`)

	fc.AddCmdOut(SSHCommDaemonArgs, `/usr/local/bin/docker --tlsverify -d -D -g /var/lib/docker -H unix:// -H tcp://0.0.0.0:2375 -b=bridge0 --registry-mirror=http://192.168.1.111:5000`)
	fc.AddTarCmd()
	sshProvider = fc.GetSshCommand

	m := getDummyMachine()
	certDir, err := RequestCertsUsingSSH(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(certDir, "dummy") {
		t.Fatalf("Expected has suffix: dummy got %s", certDir)
	}
}

func TestRequestTLSUsingSSHNoTLS(t *testing.T) {
	fc := NewFakeSshCommander()
	fc.AddCmdOut(SSHCommDaemonArgs, `/usr/local/bin/docker -d -D -g /var/lib/docker -H unix:// -H tcp://0.0.0.0:2375 -b=bridge0 --registry-mirror=http://192.168.1.111:5000`)
	sshProvider = fc.GetSshCommand

	m := getDummyMachine()
	b, err := RequestTLSUsingSSH(m)
	if err != nil {
		t.Fatal(err)
	}
	if b {
		t.Fatalf("Expected false got %b", b)
	}
}

func TestRequestTLSVerifyUsingSSHTLS(t *testing.T) {
	fc := NewFakeSshCommander()
	fc.AddCmdOut(SSHCommDaemonArgs, `/usr/local/bin/docker --tlsverify -d -D -g /var/lib/docker -H unix:// -H tcp://0.0.0.0:2375 -b=bridge0 --registry-mirror=http://192.168.1.111:5000`)
	sshProvider = fc.GetSshCommand

	m := getDummyMachine()
	b, err := RequestTLSUsingSSH(m)
	if err != nil {
		t.Fatal(err)
	}
	if !b {
		t.Fatalf("Expected true got %b", b)
	}
}

func TestRequestTLSUsingSSHTLS(t *testing.T) {
	fc := NewFakeSshCommander()
	fc.AddCmdOut(SSHCommDaemonArgs, `/usr/local/bin/docker --tls -d -D -g`)
	sshProvider = fc.GetSshCommand

	m := getDummyMachine()
	b, err := RequestTLSUsingSSH(m)
	if err != nil {
		t.Fatal(err)
	}
	if !b {
		t.Fatalf("Expected true got %b", b)
	}
}
