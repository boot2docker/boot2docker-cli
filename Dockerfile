# Dockerfile to cross compile boot2docker-cli

FROM ubuntu:13.10
MAINTAINER Riobard Zhan <me@riobard.com> (@riobard)

# Packaged dependencies
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -yq --no-install-recommends \
    build-essential ca-certificates curl git

# Install Go from binary release
RUN curl -s https://go.googlecode.com/files/go1.2.1.linux-amd64.tar.gz | tar -v -C /usr/local -xz
ENV PATH /usr/local/go/bin:$PATH

# Bootstrap Go for cross compilation (we have linux/amd64 by default)
ENV DOCKER_CROSSPLATFORMS darwin/amd64 windows/amd64
RUN cd /usr/local/go/src && bash -xc 'for platform in $DOCKER_CROSSPLATFORMS; do GOOS=${platform%/*} GOARCH=${platform##*/} ./make.bash --no-clean 2>&1; done'

ENV GOPATH  /go
ADD . /go/src/github.com/boot2docker/boot2docker-cli
WORKDIR /go/src/github.com/boot2docker/boot2docker-cli

# Download (but not install) dependencies
RUN go get -d
CMD ["make", "all"]
