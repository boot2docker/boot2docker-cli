VERSION=$(shell cat VERSION)
GOARCH=amd64
PREFIX=boot2docker-cli

default:
	@./build.sh -o $(PREFIX)-$(VERSION)

all: darwin linux windows
	@true # stop "all" from matching "%" later

%:
	@GOOS=$@ GOARCH=$(GOARCH) ./build.sh -o $(PREFIX)-$(VERSION)-$@-$(GOARCH)$(if $(filter windows, $@),.exe)

clean:
	rm $(PREFIX)*
