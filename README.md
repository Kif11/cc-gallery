## Nginx config

## Importing data

```bash
go run ./injest/main.go <insta_data_dir> <dst_dir>

# For example
go run ./injest/main.go ~/pr/instagram_data ./public/media
```

## Serving in production

```nginx
location /public/ {
    root /home/kiko/gallery/;
}
```
