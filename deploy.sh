rm -rf bin
GOOS=linux GOARCH=amd64 go build -o bin/gallery server.go
ssh -t codercat "sudo systemctl stop codercat-gallery.service"
scp bin/gallery codercat:~/gallery/gallery
ssh -t codercat "sudo systemctl start codercat-gallery.service"