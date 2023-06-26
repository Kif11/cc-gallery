rm -rf bin
GOOS=linux GOARCH=amd64 go build -o bin/gallery server.go

# Stop service
ssh -t codercat "sudo systemctl stop codercat-gallery.service"

# Sync server binary
scp bin/gallery codercat:~/gallery/gallery

# Sync assets
rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r ./public/media/ codercat:~/gallery/public/media
rsync -avu -p ./pages/ codercat:~/gallery/pages

# Restart service
ssh -t codercat "sudo systemctl start codercat-gallery.service"