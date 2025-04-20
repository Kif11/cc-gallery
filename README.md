## Codercat Gallery

This is a simple web gallery focused minimalism, simplisity and speed. It support localy stored media as well as media in s3 compatible storage.

## Importing Instagram data

```bash
go run ./injest/main.go <insta_data_folder> <dst_dir>

# For example
go run ./injest/main.go ~/pr/instagram_data ./assets/media
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

## Deployment

The gallery build into single self-contain binary that can be deployed to a server. I usually use `scp` command in conjuntion with `systemd` for persistance (see `deploy.sh` script)

## Systemd config

To make the gallery server persistent use the following `systemd` config:

```ini
[Unit]
Description=Codercat Gallery
After=network.target

[Service]
Environment=CCG_WEB_ROOT=https://cdn.codercat.xyz/gallery
Environment=CCG_S3_ENDPOINT=nyc3.digitaloceanspaces.com
Environment=CCG_S3_REGION=nyc3
Environment=CCG_S3_BUCKET=cc-storage
Environment=CCG_S3_ROOT_DIR=gallery
Environment=CCG_S3_KEY=YOUR_S3_KEY
Environment=CCG_S3_SECRET=YOUR_S3_SECRET
WorkingDirectory=/home/kiko/gallery/
Type=simple
Restart=always
RestartSec=1
User=kiko
ExecStart=/home/kiko/gallery/gallery

[Install]
WantedBy=multi-user.target
```

Note that the environmental variables in the example above are set for S3 media hosting. Here is a set of variables you need to configure when hosting images from local drive:

```sh
CCG_WEB_ROOT="/assets/media"
CCG_LOCAL_ASSET_FOLDER="/Users/kif/pr/ccgallery/assets/media"
CCG_ASSETS_FOLDER="assets"
CCG_ASSETS_URL_PREFIX="/assets"
```

## Nginx config

It usually a good idea to use application proxy such as `nginx` to be able to have multiple application on a single server. 
For hosting this gallery add config bellow to your `nging` site configuration usually located at `/etc/nginx/sites-available/`.

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

    // (Optional) for local asset hosting
    location /assets/ {
        root /home/kiko/gallery/;
    }
}
```
