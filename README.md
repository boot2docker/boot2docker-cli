# boot2docker command line management tool

This tool downloads the boot2docker ISO image, creates a VirtualBox virtual
machine, sets up two networks for that virtual machine (one NAT to allow the VM
and containers to access the internet, the other host-only to allow container
port mapping to work securely), and then provides the user a simple way to
login via SSH.

On Windows, [MSYS SSH](http://www.mingw.org/) provides a first class way to
connect to the boot2docker VM using `boot2docker.exe ssh`.

> **Note:** Docker now has an [IANA registered IP Port: 2376]( http://www.iana.org/assignments/service-names-port-numbers/service-names-port-numbers.xhtml?search=docker)
> , so the use of port 4243 is deprecated. This also means that new Boot2Docker
> ISO releases and management tool are not compatible.

## Installation

### Official Installers

Signed installers are available for [Mac OS X](http://github.com/boot2docker/osx-installer/releases) and [Windows](http://github.com/boot2docker/windows-installer/releases).

Refer to the installation instructions for [Mac OS X](http://docs.docker.io/installation/mac/) and [Windows](http://docs.docker.io/installation/windows/).

### Manual Installation

### Pre-compiled binaries

You can download binary releases at https://github.com/boot2docker/boot2docker-cli/releases

### Docker container build

The most reliable way to cross-compile boot2docker for all 3 platforms, is to
use the Dockerfile (and Docker)

```sh
git clone https://github.com/boot2docker/boot2docker-cli
cd boot2docker-cli
make
```

Built binaries will be available in the current directory.

This assumes you have an accessible Docker daemon (local or remote with `DOCKER_HOST` set)
and `make` installed.


### Install from source

You need to have the [Go compiler (v1.3 or higher)](http://golang.org) installed, and `$GOPATH`
[properly setup](http://golang.org/doc/code.html#GOPATH). Then run

    go get github.com/boot2docker/boot2docker-cli

The binary will be available at `$GOPATH/bin/boot2docker-cli`. However the
binary built this way will have missing version information when you run

    $ boot2docker version

You can solve the issue by using `make goinstall`

```sh
cd $GOPATH/src/github.com/boot2docker/boot2docker-cli
make goinstall
```

### Cross compiling

You can cross compile to OS X, Windows, and Linux. For that you need to first
[make your Go compiler ready for cross compiling to the target
platforms](http://stackoverflow.com/questions/12168873/cross-compile-go-on-osx).

Please make sure you build with golang v1.3 or later - it is required for 
`boot2docker download` to work on OS X.

We provide a Makefile to make the process a bit easier.

```sh
make darwin     # build for OS X/amd64
make linux      # build for Linux/amd64
make windows    # build for Windows/amd64
make all        # build for all three above
make clean      # clean up the built binaries
```

Built binaries will be available in the current directory.


## Usage

To initialize a new boot2docker VM, run

    $ boot2docker init

Then you can start the VM by

    $ boot2docker up

To stop the VM, run

    $ boot2docker down

And finally if you don't need the VM anymore, run

    $ boot2docker delete

to remove it completely.

You can also run commands on the remote boot2docker virtual machine:

    $ boot2docker ssh ip addr show eth1 |sed -nEe 's/^[ \t]*inet[ \t]*([0-9.]+)\/.*$/\1/p'
    192.168.59.103
    # this example is equivalent to the built in command:
    $ boot2docker ip
    192.168.59.103

In this case, the command tells you the host only interface IP address of the
boot2docker vm, which you can then use to access ports you map from your containers.

## Configuration

The `boot2docker` binary reads configuration from `$BOOT2DOCKER_PROFILE` if set, or
`$BOOT2DOCKER_DIR/profile` or `$HOME/.boot2docker/profile` or (on Windows) 
`%USERPROFILE%/.boot2docker/profile`.  `boot2docker config` will
tell you where it is looking for the file, and will also output the settings that 
are in use, so you can initialise a default file to customize using 
`boot2docker config > /home/sven/.boot2docker/profile`.

Currently you can configure the following options (undefined options take 
default values):

```ini
# Comments must be on their own lines; inline comments are not supported.

# path to VirtualBox management utility
VBM = "VBoxManage"

# path to SSH client utility
SSH = "ssh"
SSHGen = "ssh-keygen"
SSHKey = "/Users/sven/.ssh/id_boot2docker"

# name of boot2docker virtual machine
VM = "boot2docker-vm"

# URL pointing either to a Github "/releases" API endpoint to automatically
# retrieve the `boot2docker.iso` asset from the latest released version of a
# repo, or directly to an ISO image
ISOURL = "https://api.github.com/repos/boot2docker/boot2docker/releases"
#ISOURL = "https://github.com/boot2docker/boot2docker/releases/download/v1.0.0/boot2docker.iso"
#ISOURL = "https://internal.corp.org/b2d.iso"

# path to boot2docker ISO image
ISO = "/Users/sven/.boot2docker/boot2docker.iso"

# VM disk image size in MB
DiskSize = 20000

# VM memory size in MB
Memory = 2048

# host port forwarding to port 22 in the VM
SSHPort = 2022

# host port forwarding to port 2376 in the VM
DockerPort = 2376

# host-only network host IP
HostIP = "192.168.59.3"

# host only network network mask
NetMask = [255, 255, 255, 0]

# host-only network DHCP server IP
DHCPIP = "192.168.59.99"

# host-only network DHCP server enabled
DHCPEnabled = true

# host-only network IP range lower bound
LowerIP = "192.168.59.103"

# host-only network IP range upper bound
UpperIP = "192.168.59.254"
```

You can override the configurations using matching command-line flags. Type
`boot2docker -h` for more information. The configuration file options are
the same as the command-line flags with long names.



## Contribution

[![Build Status](https://travis-ci.org/boot2docker/boot2docker-cli.svg?branch=master)](https://travis-ci.org/boot2docker/boot2docker-cli)

We are implementing the same process as [Docker merge
approval](https://github.com/dotcloud/docker/blob/master/CONTRIBUTING.md#merge-approval),
so all commits need to be done via pull requests, and will need three or more
LGTMs (Looks Good To Me) before merging.

To submit pull request, please make sure to follow the [Go Style
Guide](https://code.google.com/p/go-wiki/wiki/Style). In particular, you MUST
run `gofmt` before committing. We suggest you run `go tool vet -all .` as well.

Please rebase the upstream in your fork in order to keep the commit history
tidy.
