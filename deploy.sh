rm -rf bin
GOOS=linux GOARCH=amd64 go build -o bin/gallery server.go

# Stop service
ssh -t codercat "sudo systemctl stop codercat-gallery.service"

# Sync server binary
scp bin/gallery codercat:~/gallery/gallery

# Print disc usage
ssh -t codercat "df -h --total"

# Sync assets
rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r ./public/media/kif/ codercat:~/gallery/public/media/kif
rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r --exclude 'story_*' ./public/media/snay/ codercat:~/gallery/public/media/snay
rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r ./public/gallery/ codercat:~/gallery/public/gallery
rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r ./public/*.css codercat:~/gallery/public

rsync -avu --delete -p ./pages/ codercat:~/gallery/pages

# Restart service
ssh -t codercat "sudo systemctl start codercat-gallery.service"