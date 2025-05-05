# (Optional) Sync images and video files if using local gallery
# rsync -avu -p --chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r ./assets/media/kif/ user@server:~/gallery/assets/media/

# Make sure we are doing clean build
rm -rf bin

# Build for linux amd64
GOOS=linux GOARCH=amd64 go build -o bin/gallery server.go

# Copy server binary to remote host
scp bin/gallery codercat:~/gallery/gallery.new

# Swap to a new binary and restart the service
ssh -t codercat "
mv ~/gallery/gallery.new ~/gallery/gallery
chmod +x ~/gallery/gallery
sudo systemctl restart codercat-gallery.service
"
