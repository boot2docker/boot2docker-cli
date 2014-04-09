version=`cat VERSION`
gitSha=`git rev-parse --short HEAD`

default:
	@go build -ldflags "-X main.Version $(version) -X main.GitSHA $(gitSha)" -o boot2docker-cli-$(version)

all: darwin linux windows

darwin:
	@GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version $(version) -X main.GitSHA $(gitSha)" -o boot2docker-cli-$(version)-darwin-amd64

linux:
	@GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version $(version) -X main.GitSHA $(gitSha)" -o boot2docker-cli-$(version)-linux-amd64

windows:
	@GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version $(version) -X main.GitSHA $(gitSha)" -o boot2docker-cli-$(version)-windows-amd64.exe
