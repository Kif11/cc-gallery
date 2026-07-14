#!/bin/bash

# Function to rename files in a directory using exif date
# Usage: rename_files <directory> <type>
rename_files() {
    local dir="$1"
    local type="$2"

    if [[ ! -d "$dir" ]]; then
        echo "Error: $dir is not a directory"
        return 1
    fi

    if [[ -z "$type" ]]; then
        echo "Error: type is required"
        return 1
    fi

    for file in "$dir"/*; do
        if [[ -f "$file" ]]; then
            # Get file extension
            ext="${file##*.}"

            # Get unix timestamp from exif DateTimeOriginal
            seconds=$(exiftool -s3 -DateTimeOriginal -d "%s" "$file" 2>/dev/null)
            if [[ "$seconds" =~ ^[0-9]+$ ]]; then
                timestamp=$((seconds * 1000))
            else
                echo "Warning: No valid exif date for $file, using file modification time"
                # If no exif date or invalid, use file modification time in milliseconds
                timestamp=$(( $(stat -f %m "$file") * 1000 ))
            fi

            # New filename: type_timestamp_0.ext
            newname="${type}_${timestamp}_0.${ext}"

            # Rename the file
            mv "$file" "$dir/$newname"
            echo "Renamed $file to $dir/$newname"
        fi
    done
}

rename_files "$1" "$2"
