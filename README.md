# Static Site For Serving Video Content

Why does serving up your home media library require a database and manually curating meta data about each video? Do you want a solution where you just render a webpage of links to play a directory of videos (recursive) with previews.

This golang command line app creates crawls a recursively crawls a directory of video files, creates a thumbnail directory with a preview for each video and renders and `index.html` for each video that can be served on any web server.

Example:

``` bash
go run main.go
docker run -p 80:80 -v `pwd`:/usr/share/nginx/html
# enjoy your home media server
```

Preview:
![preview][imgs/preview1.png]
![preview][imgs/preview2.png]

External Dependencies:
- bash
- ffmpeg
