## Importing data

```bash
go run ./injest/main.go <insta_data_folder> <dst_dir>

# For example
go run ./injest/main.go ~/pr/instagram_data ./public/media
```

Note: `insta_data_folder` directory structure should look like this:

```
instagram_data_archive/
    <user>/
        content/
            posts_1.json
            ...
        media/
            reels/
            posts/
            ...
        ...
```

## Serving in production

```nginx
location /public/ {
    root /home/kiko/gallery/;
}
```

## Nginx config
Add config bellow to `/etc/nginx/sites-available/codercat.xyz`.

```nginx
server {
    
    ...
    
    location /gallery/ {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    location /public/ {
        root /home/kiko/gallery/;
    }
}
```
