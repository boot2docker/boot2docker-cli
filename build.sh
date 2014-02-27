#!/bin/sh
set -e

version="$(cat VERSION)"
gitSha="$(git rev-parse --short HEAD)"

set -x
exec go build -ldflags "-X main.Version $version -X main.GitSHA $gitSha"
