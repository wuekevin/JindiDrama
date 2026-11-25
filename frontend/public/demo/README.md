演示媒体由本项目用 ffmpeg 的 `testsrc2` 测试图案和正弦音生成，每段约 24 秒，可自由分发。

- 高清为原始的 360×640 MP4 和 WebM。
- 流畅为同一片段降采样生成的 240×426 MP4 和 WebM，文件名带 `-sd`。

演示下载按设置选用对应的内置 MP4，默认高清。

以下命令从原始片段生成对应流畅版本。

```sh
for clip in 1 2 3; do
  ffmpeg -i "demo-${clip}.mp4" -vf 'scale=240:426,setsar=1' \
    -c:v libx264 -preset veryfast -crf 28 -c:a copy "demo-${clip}-sd.mp4"
  ffmpeg -i "demo-${clip}.webm" -vf 'scale=240:426,setsar=1' \
    -c:v libvpx-vp9 -deadline good -cpu-used 6 -crf 40 -b:v 0 -c:a copy "demo-${clip}-sd.webm"
done
```
