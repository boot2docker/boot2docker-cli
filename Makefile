
all:
	GOOS=linux GOARCH=386 go build -o bin/b2d_linux_386 &
	GOOS=linux GOARCH=amd64 go build -o bin/b2d_linux_amd64 &
	GOOS=darwin GOARCH=386 go build -o bin/b2d_darwin_386 &
	GOOS=darwin GOARCH=amd64 go build -o bin/b2d_darwin_amd64 &
	GOOS=windows GOARCH=386 go build -o bin/b2d_windows_386 &
	GOOS=windows GOARCH=amd64 go build -o bin/b2d_windows_amd64 &

clean:
	rm -r bin/
