# go-boot2docker

This is the Go port of the boot2docker
(https://github.com/boot2docker/boot2docker) management script. It is intended
to replace the shell version because the shell script cannot be used on Windows
without cygwin. 

The Go port will produce a single binary without extra dependencies for the
following platforms:

- Linux/386
- Linux/amd64
- OS X (Darwin)/386
- OS X (Darwin)/amd64
- Windows/386
- Windows/amd64


## Build

Once you have the necessary Go toolchain, run

    make
