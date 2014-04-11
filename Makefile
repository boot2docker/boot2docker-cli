VERSION=$(shell cat VERSION)
GITSHA1=$(shell git rev-parse --short HEAD)
GOARCH=amd64
GOFLAGS=-ldflags "-X main.Version $(VERSION) -X main.GitSHA $(GITSHA1)"
PREFIX=boot2docker-cli

# This binary will be availabe in the current directory.
default:
	go build $(GOFLAGS) -o $(PREFIX)-$(VERSION)

# This binary will be installed at $GOBIN or $GOPATH/bin.
install:
	go install $(GOFLAGS)

all: darwin linux windows
	@true # stop "all" from matching "%" later

%:
	GOOS=$@ GOARCH=$(GOARCH) go build $(GOFLAGS) -o $(PREFIX)-$(VERSION)-$@-$(GOARCH)$(if $(filter windows, $@),.exe)

clean:
	rm $(PREFIX)*

# the `|| true` hack is a temporary fix for https://github.com/dotcloud/docker/issues/3986
dockerbuild:
	docker build -t boot2docker-golang .
	docker run --name boot2docker-cli-build boot2docker-golang
	docker cp boot2docker-cli-build:/go/src/github.com/boot2docker/boot2docker-cli/$(PREFIX)-$(VERSION)-darwin-$(GOARCH) . || true
	docker cp boot2docker-cli-build:/go/src/github.com/boot2docker/boot2docker-cli/$(PREFIX)-$(VERSION)-linux-$(GOARCH) . || true
	docker cp boot2docker-cli-build:/go/src/github.com/boot2docker/boot2docker-cli/$(PREFIX)-$(VERSION)-windows-$(GOARCH).exe . || true
	docker rm boot2docker-cli-build

# In case dockerbuild does not finish correctly and you want to remove the
# `boot2docker-cli-build` container.
dockerbuild-clean:
	docker rm boot2docker-cli-build
