# Make sure we are doing clean build
rm -rf bin

# Build for linux amd64
GOOS=linux GOARCH=amd64 go build -o bin/gallery server.go

# Stop remote service
ssh -t user@server "sudo systemctl stop ccgallery.service"

# Copy server binary to remote host
scp bin/gallery user@server:~/bin/gallery

# (Optional) Sync images and video files if using local gallery
# rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r ./assets/media/kif/ user@server:~/gallery/assets/media/

# Restart service
ssh -t user@server "sudo systemctl start ccgallery.service"