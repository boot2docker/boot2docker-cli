# boot2docker-cli

This is the Go port of the boot2docker (https://github.com/boot2docker/boot2docker)
management script. It is intended to replace the shell script eventually. It is
currently usable but since it is under active development, frequent changes and
bugs are expected. USE AT YOUR OWN RISK.

The Go port will produce a single binary without extra dependencies for the
following platforms:

- Linux/386
- Linux/amd64
- OS X (Darwin)/386
- OS X (Darwin)/amd64
- Windows/386
- Windows/amd64


## Installation

Once you have your `$GOPATH` [properly
setup](http://golang.org/doc/code.html#GOPATH), run

    go get github.com/boot2docker/boot2docker-cli


The binary will be available at `$GOPATH/bin/boot2docker-cli`.

If you don't want to install golang, you can use the `Dockerfile` to create the
binary for any supported target platform.

1. Build the image with the Go toolchain: `docker build -t boot2docker-golang .`
2. Choose the right settings for your plattform:
  * Windows: `GOOS=windows GOARCH=amd64`
  * OS X: `GOOS=darwin GOARCH=amd64`
  * Linux: `GOOS=linux GOARCH=amd64`
3. Build the binaries: (if building for Windows, don't forget to add `.exe` to
   the end of the binary name in the arguments to the `docker cp` line below)
```sh
docker run -e GOOS=darwin -e GOARCH=amd64 --name boot2docker-buildcli boot2docker-golang
docker cp boot2docker-buildcli:/go/src/github.com/boot2docker/boot2docker-cli/boot2docker-cli .
docker rm boot2docker-buildcli
# and test it:
./boot2docker-cli version
```

The binary `boot2docker-cli` will be in your current folder.


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

## What boot2docker-cli does

This tool downloads the boot2docker.iso, creates a virtual machine, sets up 2 
networks for that virtual machine (one NAT to allow the VM and containers to access
the internet, the other to allow container port mapping to work securely), and then 
provides the user a simple way to connect to the vm using an external ssh client.

On Windows, [MSYS ssh](http://www.mingw.org/) provides a first class way to connect
to the boo2docker vm (using ``boot2docker-cli.exe ssh``.

## Configuration

The `boot2docker-cli` binary reads configuration from `$BOOT2DOCKER_PROFILE`, or
if not found, from `$BOOT2DOCKER_DIR/profile`. Currently you can configure
the following options (undefined options take default values):

    VBM=VBoxManage                  # path to the VirtualBox management utility
    SSH=ssh                         # path to the `ssh` client utility
    VM=boot2docker-vm               # name of the boot2docker virtual machine in VirtualBox,
    Dir=$HOME/.boot2docker          # path to the boot2docker config directory
    ISO=$BOOT2DOCKER_DIR/boot2docker.iso    # path to the boot2docker ISO image
    Disk=$BOOT2DOCKER_DIR/boot2docker.vmdk  # path to the boot2docker disk image
    DiskSize=20000                  # boot2docker disk image size in MB
    Memory=1024                     # boot2docker VM memory size in MB
    SSHPort=2022                    # host port forwarding to port 22 in the VM
    DockerPort=4243                 # host port forwarding to port 4243 in the VM
    HostIP=192.168.59.3             # host-only network host IP
    DHCPIP=192.168.59.99            # host-only network DHCP server IP
    NetworkMask=255.255.255.0       # host only network network mask
    LowerIPAddress=192.168.59.103   # host-only network IP range lower bound
    UpperIPAddress=192.168.59.254   # host-only network IP range upper bound
    DHCPEnabled=Yes                 # host-only network's DHCP Server enabled

Environment variables in the profile will be expanded.

You can override the configurations using command line flags. Type
`boot2docker-cli -h` for more information. 



## Contribution

We are implementing the same process as [Docker merge
approval](https://github.com/dotcloud/docker/blob/master/CONTRIBUTING.md#merge-approval),
so all commits need to be done via pull requests, and will need three or more
LGTMs (Looks Good To Me) before merging.

If you want to submit pull request, please make sure you follow the [Go Style
Guide](https://code.google.com/p/go-wiki/wiki/Style). In particular, you MUST
run `gofmt` before committing. We suggest you run `go tool vet -all .` as well.
