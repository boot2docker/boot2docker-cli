#!/usr/bin/env bash

# Set version to latest unless set by user
if [ -z "$VERSION" ]; then
  VERSION="1.0.1"
fi

echo "Dowloading version ${VERSION}..."

# OS information (contains e.g. Darwin x86_64)
UNAME=`uname -a`
# Determine platform
if [[ $UNAME == *"Darwin"* ]]; then
  PLATFORM="darwin"
elif [[ $UNAME == *"Cygwin"* ]]; then
  PLATFORM="windows"
else
  PLATFORM="linux"
fi
# Determine architecture
if [[ ($UNAME == *x86_64*) || ($UNAME == *amd64*) ]]
then
  ARCH="amd64"
else
  echo "Currently, there are no 32bit binaries provided."
  echo "You will need to go get / go install github.com/boot2docker/boot2docker-cli."
  exit 1
fi

# Download binary
curl -L -o boot2docker "https://github.com/boot2docker/boot2docker-cli/releases/download/v${VERSION}/boot2docker-v${VERSION}-${PLATFORM}_${ARCH}"

# Make binary executable
chmod +x boot2docker

echo "Done."
