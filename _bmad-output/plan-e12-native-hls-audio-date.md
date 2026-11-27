---
title: 'E12 苹果 CMS 双音轨时间戳的 Apple Silicon 原生验收'
type: chore
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '2fc7ebca07f1a88b98f696d6372622ca2d982f04'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

在 Apple Silicon 原生 WKWebView 验证苹果 CMS 目录和带节目时间戳的 HLS 双音轨点播。用实际前端及 Go 中继打开目录和分集，确认默认音轨、切换音轨后的解码和进度、正常退出后的 SQLite 完整性。将验收边界、结果和未完成环境记录到 STATUS.md。

</frozen-after-approval>

## Implementation Notes

24 秒自制 H.264 与两路 AAC，由 FFmpeg 生成主清单及分段。测试构建使用隔离 CMS 传输和 WebView 观察脚本，保留生产目录、租约发布、HLS 清单改写、分段校验、浏览器播放与历史保存逻辑。普通话上游清单采用小数逗号，Go 中继改为小数点后交给原生播放器。使用独立数据目录。

## Verification

- 原生测试的清单和资源请求须包含主清单、视频清单、两路音频清单、三份初始化文件以及各路分段。
- WKWebView 使用本机内部地址加载 HLS，识别两路音轨，默认普通话。切换粤语后同一视频继续解码，正常退出并保存该集进度。SQLite 完整性检查和退出标记清理通过。

## Results

- AppKit 实机在原生 WKWebView 解码 384×216、24 秒的自制 H.264 点播素材。原生媒体选择菜单显示「普通话」和「粤语」，默认普通话。实际鼠标点击粤语后，AudioTrack 探针确认粤语启用、普通话取消，画面从 4.563 秒继续至 6.600 秒，累计 146 帧。验收保持静音，范围为轨道选择与画面解码。
- 生产 Go 中继从限定的合成传输读取视频和两路音频清单、初始化文件及媒体分段。两路 AAC 素材独立解码分别测得 440 Hz 和 660 Hz。
- 退出前存在一个活动会话标记，SQLite 已保存第 1 集 5012 ms。正常退出后读回 6747 ms、总长 24000 ms，完整性为 ok，会话标记和 SQLite 临时文件清理通过。在原生退出前关闭验收脚本的只读连接，避免干扰数据库收尾。
- 原版探针等待预加载完成时未推动原生媒体下载。最终显式开始播放，用实际鼠标操作原生菜单，并激活窗口后读取辅助功能树。Go 不可将单切片直接写入现有 Universal 主程序。测试主程序单独构建后替换隔离副本，并重新施加临时签名。
- 此次为使用本地 CMS 传输和观察探针的测试应用。正式签名成品、实际授权公网片源和其他平台待验收。

## Review Triage Log

- 中，已修复。实际鼠标打开原生音轨菜单并选择粤语，保留菜单树和语言状态。记录静音边界。
- 低，已修复。说明小数逗号属于上游清单，与 Go 中继返回的小数点区分。
- 中，已修复。退出前读取数据库，要求最终进度高于旧值并达到切换后显示的进度。
- 低，已修复。退出前记录活动会话标记，退出后验证已清除。
- 中，已修复。清理试跑中记录的系统菜单内容，脚本保留应用窗口子树。
