#!/usr/bin/env bash

set -e

# Set version to latest unless set by user
if [ -z "$VERSION" ]; then
  VERSION=$(<VERSION)
fi
EXTENSION=""

echo "Downloading version ${VERSION}..."

# OS information (contains e.g. Darwin x86_64)
UNAME=`uname -a`
# Determine platform
if [[ $UNAME == *"Darwin"* ]]; then
  PLATFORM="darwin"
elif [[ ($UNAME == *MINGW*) || ($UNAME == *Cygwin*) ]]; then
  PLATFORM="windows"
  EXTENSION=".exe"
  UNAME="${PROCESSOR_ARCHITEW6432}"
else
  PLATFORM="linux"
fi
# Determine architecture
if [[ ($UNAME == *x86_64*) || ($UNAME == *amd64*) || ($UNAME == *AMD64*) ]]
then
  ARCH="amd64"
else
  echo "Currently, there are no 32bit binaries provided."
  echo "You will need to go get / go install github.com/boot2docker/boot2docker-cli."
  exit 1
fi

# Download binary
URL="https://github.com/boot2docker/boot2docker-cli/releases/download/v${VERSION}/boot2docker-v${VERSION}-${PLATFORM}-${ARCH}${EXTENSION}"
echo "Downloading $URL"
curl -L -o "boot2docker${EXTENSION}" "$URL"

# Make binary executable
chmod +x "boot2docker${EXTENSION}"

echo "Done."
