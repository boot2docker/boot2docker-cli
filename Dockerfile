# Dockerfile to cross compile boot2docker-cli

FROM golang:1.3-cross
RUN apt-get update && apt-get install -y make

ADD . /go/src/github.com/boot2docker/boot2docker-cli
WORKDIR /go/src/github.com/boot2docker/boot2docker-cli

# Download (but not install) dependencies
RUN go get -d -v ./...

CMD ["make", "all"]
