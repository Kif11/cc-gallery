# post "kif/2023" <img.jpg>
# Required tools:
# - ImageMagic
# - ffmpeg

set -e

# On Unix the command should be `stat -c %W "$file_path"`` see https://unix.stackexchange.com/questions/91197/how-can-get-the-creation-date-of-a-file
creation_time=$(stat -f%B "$2")
tmpdir="./tmp"
mkdir -p "$tmpdir"
mime_type=$(file -b --mime-type "$2")

# Check if the MIME type indicates a video
if [[ $mime_type == video/* ]]; then

    new_name="reel_${creation_time}_0.mp4"
    new_path="$tmpdir/$new_name"
    
    if test -e ${new_path}; then
        echo "Reusing existing file ${new_path}"
    else
        ffmpeg -y -i "$2" -c:v libx264 -pix_fmt yuv420p -b:v 5M -vf scale="1920:trunc(ow/a/2)*2" -profile:v main -c:a aac -b:a 128k "$new_path"
    fi

# Check if the MIME type indicates an image
elif [[ $mime_type == image/* ]]; then

    new_name="post_${creation_time}_0.jpg"
    new_path="$tmpdir/$new_name"

    convert -format webp -quality 75 -resize 1920 "$2" "$new_path"
    
else

    echo "File type is unknown."
    exit 1

fi

du -sh "$new_path"

scp "$new_path" codercat:/home/kiko/gallery/public/media/$1
insta "$2"