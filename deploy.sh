rm -rf bin
GOOS=linux GOARCH=amd64 go build -o bin/gallery server.go

# Stop service
ssh -t codercat "sudo systemctl stop codercat-gallery.service"

# Sync server binary
scp bin/gallery codercat:~/gallery/gallery

# (Optional) Sync images and video files if using local gallery
# rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r ./assets/media/kif/ codercat:~/gallery/assets/media/kif
# rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r --exclude 'story_*' ./assets/media/snay/ codercat:~/gallery/assets/media/snay

# Restart service
ssh -t codercat "sudo systemctl start codercat-gallery.service"