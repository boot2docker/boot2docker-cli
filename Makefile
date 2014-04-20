VERSION := $(shell cat VERSION)
GITSHA1 := $(shell git rev-parse --short HEAD)
GOARCH := amd64
GOFLAGS := -ldflags "-X main.Version $(VERSION) -X main.GitSHA $(GITSHA1)"
PREFIX := boot2docker-cli
DOCKER_IMAGE := boot2docker-golang
DOCKER_CONTAINER := boot2docker-cli-build
DOCKER_SRC_PATH := /go/src/github.com/boot2docker/boot2docker-cli


default: dockerbuild
	@true # stop from matching "%" later


# Build binaries in Docker container. The `|| true` hack is a temporary fix for
# https://github.com/dotcloud/docker/issues/3986
dockerbuild:
	docker build -t "$(DOCKER_IMAGE)" .
	docker run --name "$(DOCKER_CONTAINER)" "$(DOCKER_IMAGE)" 
	docker cp "$(DOCKER_CONTAINER)":"$(DOCKER_SRC_PATH)"/$(PREFIX)-$(VERSION)-darwin-$(GOARCH) . || true
	docker cp "$(DOCKER_CONTAINER)":"$(DOCKER_SRC_PATH)"/$(PREFIX)-$(VERSION)-linux-$(GOARCH) . || true
	docker cp "$(DOCKER_CONTAINER)":"$(DOCKER_SRC_PATH)"/$(PREFIX)-$(VERSION)-windows-$(GOARCH).exe . || true
	docker rm "$(DOCKER_CONTAINER)"


# Remove built binaries and Docker container. Silent errors if container not found.
clean:
	rm -f $(PREFIX)*
	docker rm "$(DOCKER_CONTAINER)" 2>/dev/null || true


all: darwin linux windows
	@true # stop "all" from matching "%" later


# Native Go build per OS/ARCH combo.
%:
	GOOS=$@ GOARCH=$(GOARCH) go build $(GOFLAGS) -o $(PREFIX)-$(VERSION)-$@-$(GOARCH)$(if $(filter windows, $@),.exe)


# This binary will be installed at $GOBIN or $GOPATH/bin. Requires proper
# $GOPATH setup AND the location of the source directory in $GOPATH.
goinstall:
	go install $(GOFLAGS)
