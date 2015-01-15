# Dockerfile to cross compile boot2docker-cli

FROM golang:1.4-cross

WORKDIR /go/src/github.com/boot2docker/boot2docker-cli

# Download (but not install) dependencies
RUN go get -v github.com/BurntSushi/toml
RUN go get -v github.com/ogier/pflag

ADD . /go/src/github.com/boot2docker/boot2docker-cli

CMD ["make", "all"]
