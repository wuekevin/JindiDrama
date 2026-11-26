# HLS 播放测试样片

`blue.mpegts` 是 2 秒蓝色画面，使用 H.264 和 MPEG-TS 容器。浏览器测试构造清单并读取这份样片，运行时无需 FFmpeg。

在仓库根目录运行以下命令可重新生成样片。

```sh
ffmpeg -hide_banner -loglevel error -y -f lavfi -i 'color=c=blue:s=64x36:r=10' \
  -t 2 -an -c:v libx264 -preset ultrafast -pix_fmt yuv420p -g 10 -f mpegts \
  frontend/tests/fixtures/hls/blue.mpegts
```
