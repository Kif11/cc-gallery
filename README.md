## Nginx config

## Importing data

```bash
go run ./injest/main.go <insta_data_folder> <dst_dir>

# For example
go run ./injest/main.go ~/pr/instagram_data ./public/media
```

Note: `insta_data_folder` directory structure should look like this:

```
instagram_data_archive
    user
        content
        media
        ...

```

## Serving in production

```nginx
location /public/ {
    root /home/kiko/gallery/;
}
```
