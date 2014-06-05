#!/bin/sh
set -e

# This is how I built the 0.9.2 boot2docker-cli release on my mac.

# install https://storage.googleapis.com/golang/go1.2.2.darwin-amd64-osx10.8.pkg

TMP=$(mktemp -d /tmp/b2d-cli.XXXXXX)
echo Building in $TMP
export GOPATH=$TMP
export DOCKER_HOST=tcp://localhost:2375
go get github.com/boot2docker/boot2docker-cli
cd ${TMP}/src/github.com/boot2docker/boot2docker-cli


make

echo building OSX native
make darwin

pwd
