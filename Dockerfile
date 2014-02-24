# Dockerfile for go cross compile boot2docker-cli
#
# docker build -t golang .
# 
# change GOOS and GOARCH to your target plattform
# docker run -e GOOS=darwin -e GOARCH=amd64 --name boot2docker-cli golang
# docker cp boot2docker-cli:/data/boot2docker boot2docker/boot2docker
# docker rm boot2docker-cli

 
FROM ubuntu:13.10
MAINTAINER Andreas Heissenberger <andreas@heissenberger.at> (@aheissenberger)

# Packaged dependencies
RUN	apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -yq \
	automake \
	build-essential \
	curl \
	ca-certificates \
	git \
	mercurial \
	--no-install-recommends

# Install Go
RUN	curl -s https://go.googlecode.com/files/go1.2.src.tar.gz | tar -v -C /usr/local -xz
ENV	PATH	/usr/local/go/bin:$PATH
ENV	GOPATH	/go:/go/src/github.com/dotcloud/docker/vendor
RUN	cd /usr/local/go/src && ./make.bash --no-clean 2>&1

# Compile Go for cross compilation
ENV	DOCKER_CROSSPLATFORMS	linux/386 linux/arm darwin/amd64 darwin/386 windows/386 windows/amd64
# (set an explicit GOARM of 5 for maximum compatibility)
ENV	GOARM	5
RUN	cd /usr/local/go/src && bash -xc 'for platform in $DOCKER_CROSSPLATFORMS; do GOOS=${platform%/*} GOARCH=${platform##*/} ./make.bash --no-clean 2>&1; done'

RUN mkdir -p /data
WORKDIR /data
ADD boot2docker /data

CMD ["/bin/sh","-c","go build -o boot2docker"]
