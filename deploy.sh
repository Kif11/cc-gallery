GOOS=linux GOARCH=amd64 go build -o bin/gallery server.go
scp bin/gallery codercat:~/gallery/gallery