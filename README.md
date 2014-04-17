# boot2docker-cli

This is the Go port of the
[boot2docker](https://github.com/boot2docker/boot2docker) [management
script](https://github.com/boot2docker/boot2docker/blob/master/boot2docker). It
is intended to replace the shell script eventually. Currently the Go port is
usable but since it is under active development, frequent changes and bugs are
to be expected. USE AT YOUR OWN RISK.

## What it does

This tool downloads the boot2docker ISO image, creates a VirtualBox virtual
machine, sets up two networks for that virtual machine (one NAT to allow the VM
and containers to access the internet, the other host-only to allow container
port mapping to work securely), and then provides the user a simple way to
login via SSH.

On Windows, [MSYS SSH](http://www.mingw.org/) provides a first class way to
connect to the boot2docker VM using `boot2docker-cli.exe ssh`.


## Installation

### Pre-compiled binaries

You can dowload binary releases at https://github.com/boot2docker/boot2docker-cli/releases

### Install from source

You need to have [Go compiler](http://golang.org) installed, and `$GOPATH`
[properly setup](http://golang.org/doc/code.html#GOPATH). Then run

    go get github.com/boot2docker/boot2docker-cli

The binary will be available at `$GOPATH/bin/boot2docker-cli`. However the
binary built this way will have missing version information when you run

    boot2docker-cli version

You can solve the issue by using `make goinstall`

```sh
cd $GOPATH/src/github.com/boot2docker/boot2docker-cli
make goinstall
```

### Cross compiling

You can cross compile to OS X, Windows, and Linux. For that you need to first
[make your Go compiler ready for cross compiling to the target
platforms](http://stackoverflow.com/questions/12168873/cross-compile-go-on-osx).

We provide a Makefile to make the process a bit easier.

```sh
make darwin     # build for OS X/amd64
make linux      # build for Linux/amd64
make windows    # build for Windows/amd64
make all        # build for all three above
make clean      # clean up the built binaries
```

Built binaries will be available in the current directory.


### Docker build

You can also build in a Docker container.

```sh
make dockerbuild
```

Built binaries will be available in the current directory.


### Caveats

Currently the binary cross-compiled from Windows/Linux to OS X has a [TLS
issue](https://github.com/boot2docker/boot2docker-cli/issues/11), and as a
result

    boot2docker-cli download

will fail. You need to do a native OS X build to avoid this problem.


## Usage

To initialize a new boot2docker VM, run

    boot2docker-cli init

Then you can start the VM by

    boot2docker-cli up

To stop the VM, run

    boot2docker-cli down

And finally if you don't need the VM anymore, run

    boot2docker-cli delete

to remove it completely.


## Configuration

The `boot2docker-cli` binary reads configuration from `$BOOT2DOCKER_PROFILE`, or
if not found, from `$BOOT2DOCKER_DIR/profile`. Currently you can configure
the following options (undefined options take default values):

```ini
# Comments must be on their own lines; inline comments are not supported.

# path to VirtualBox management utility
vbm=VBoxManage

# path to SSH client utility
ssh=ssh

# name of boot2docker virtual machine
vm=boot2docker-vm

# path to boot2docker config directory
dir=$HOME/.boot2docker

# path to boot2docker ISO image
iso=$BOOT2DOCKER_DIR/boot2docker.iso

# VM disk image size in MB
disksize=20000

# VM memory size in MB
memory=1024

# host port forwarding to port 22 in the VM
sshport=2022

# host port forwarding to port 4243 in the VM
dockerport=4243

# host-only network host IP
hostip=192.168.59.3

# host only network network mask
netmask=255.255.255.0

# host-only network DHCP server IP
dhcpip=192.168.59.99

# host-only network DHCP server enabled
dhcp=true

# host-only network IP range lower bound
lowerip=192.168.59.103

# host-only network IP range upper bound
upperip=192.168.59.254
```

Environment variables of the form `$ENVVAR` in the profile will be expanded,
even on Windows.

You can override the configurations using command-line flags. Type
`boot2docker-cli -h` for more information. The configuration file options are
the same as the command-line flags with long names.



## Contribution

We are implementing the same process as [Docker merge
approval](https://github.com/dotcloud/docker/blob/master/CONTRIBUTING.md#merge-approval),
so all commits need to be done via pull requests, and will need three or more
LGTMs (Looks Good To Me) before merging.

To submit pull request, please make sure to follow the [Go Style
Guide](https://code.google.com/p/go-wiki/wiki/Style). In particular, you MUST
run `gofmt` before committing. We suggest you run `go tool vet -all .` as well.

Please rebase the upstream in your fork in order to keep the commit history
tidy.
