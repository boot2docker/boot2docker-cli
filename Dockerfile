# Dockerfile to cross compile boot2docker-cli

FROM debian:jessie

# Packaged dependencies
RUN apt-get update && apt-get install -y build-essential curl git

# Install Go from binary release
RUN curl -sSL http://golang.org/dl/go1.3.src.tar.gz | tar -xzC /usr/local
ENV PATH /usr/local/go/bin:$PATH

# Bootstrap Go for cross compilation (we have linux/amd64 by default)
ENV DOCKER_CROSSPLATFORMS darwin/amd64 windows/amd64
RUN cd /usr/local/go/src && bash -xec 'for platform in $DOCKER_CROSSPLATFORMS; do GOOS=${platform%/*} GOARCH=${platform##*/} ./make.bash --no-clean 2>&1; done'

ENV GOPATH /go
ADD . /go/src/github.com/boot2docker/boot2docker-cli
WORKDIR /go/src/github.com/boot2docker/boot2docker-cli

# Download (but not install) dependencies
RUN go get -d -v

CMD ["make", "all"]
