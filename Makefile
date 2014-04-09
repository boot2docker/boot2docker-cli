version=`cat VERSION`
gitSha=`git rev-parse --short HEAD`
GOARCH=amd64
GOBUILD=GOARCH=$(GOARCH) go build -ldflags "-X main.Version $(version) -X main.GitSHA $(gitSha)"
PREFIX=boot2docker-cli

.PHONY: default all darwin linux windows clean

default:
	@$(GOBUILD) -o $(PREFIX)-$(version)

all: darwin linux windows

darwin:
	@GOOS=$@ $(GOBUILD) -o $(PREFIX)-$(version)-$@-$(GOARCH)

linux:
	@GOOS=$@ $(GOBUILD) -o $(PREFIX)-$(version)-$@-$(GOARCH)

windows:
	@GOOS=$@ $(GOBUILD) -o $(PREFIX)-$(version)-$@-$(GOARCH).exe

clean:
	rm $(PREFIX)*
