---
title: 'E04 演示视频连续快进与书签'
type: bugfix
ticket: ''
created: '2026-11-28'
status: built
baseline_revision: '413f273b23b821033c93b228d46de897d4e46396'
route: oneshot
route_source: pinned
risk: medium
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent">

## Intent

暂停的真实演示视频从 12 秒连续两次快进，有时停在 22 秒，导致播放器没有兑现第二次快进。快速连续操作应累加到最新目标，保持暂停，保存最新用户选择的进度。换集、重新加载和退出时，旧媒体的异步事件不得污染新一集或错误标记为看完。

</frozen-after-approval>

## Implementation Notes

- 原构建收到两次快进，第二次目标 24.0035 秒超出 WebM 视频轨约 23.999 秒的结束位置，WebKit 回到 22 秒。FFprobe 核对了 12 个内置媒体文件。仅将演示视频手动定位限制在末尾前 40 毫秒的实验仍然复现重叠定位回退。
- 在媒体定位完成事件后提交最新请求。检测到目标未到达时最多再提交一次。播放器在等待期间用最新目标累计快进，切换画质时继续使用最新目标。更换剧集与重载清除旧请求。
- 单测模拟已有定位尚未完成、一次媒体回退及重试耗尽，验证有限重试、暂停与 99% 书签。真实视频在定位完成并有当前帧数据时核对时间，同时复验离开后重新打开。
- Chromium 快进与进度条用例改为等待原定时间，断言的定位范围和书签语义保持原值。

## Results

- Chromium 与 WebKit 同一生产构建各通过全部 410 项，均一次通过。真实播放、暂停和重开同一剧集的 WebKit 专项连续通过 32 次。
- Apple Silicon 原生测试从 12 秒暂停状态连按两次右键，退出后 SQLite 保存第 80 集 23960/24000 毫秒和 99%，完整性正常。Go 全量、vet、Windows x64 交叉编译及 Vitest 128 项通过。生产依赖和两个已构建二进制的扫描未发现已知漏洞。

## Verification

- 前端类型检查、构建、Vitest。
- 原真实视频场景重复测试，覆盖 Chromium 和 WebKit 播放器及相关全量回归。
- Go 全量验证及 vet。

## Review Triage Log

- medium，已修复。同步拒绝和异步回退均按同一上限重试。暂停时两次定位都未命中，显示媒体实际落点并保留所选书签。播放恢复或再选位置时清除失败目标。单测覆盖同步拒绝、异步拒绝与重试成功。
- medium，已修复。手动和书签恢复使用同一片尾余量。MP4 的最后一帧从约 23.958 秒开始，旧恢复上限 23.950 秒会向前退一帧。
- medium，已修复验收。原用例现在等待媒体完成定位并能提供当前帧数据，再核对片尾时间。原生测试核对持续暂停的 UI 和 SQLite 记录，实际媒体落点另由真实视频浏览器用例核对。
