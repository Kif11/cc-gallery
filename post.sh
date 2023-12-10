# post "kif/2023" <img.jpg>
set -e

# On Unix the command should be `stat -c %W "$file_path"`` see https://unix.stackexchange.com/questions/91197/how-can-get-the-creation-date-of-a-file
creation_time=$(stat -f%B "$2")
tmpdir="./tmp"
mkdir -p ${tmpdir}

new_name="post_${creation_time}_0.jpg"
new_path="${tmpdir}/${new_name}"

convert -format webp -quality 75 -resize 1920 $2 ${new_path}

du -sh ${new_path}

scp ${new_path} codercat:/home/kiko/gallery/public/media/$1
