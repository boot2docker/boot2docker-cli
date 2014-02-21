all:
	GOOS=linux GOARCH=386 go install
	GOOS=linux GOARCH=amd64 go install
	GOOS=darwin GOARCH=386 go install
	GOOS=darwin GOARCH=amd64 go install
	GOOS=windows GOARCH=386 go install
	GOOS=windows GOARCH=amd64 go install
