---
title: 'E13 随包说明包含授权 CMS 片源配置'
type: bug
ticket: ''
created: '2026-11-27'
status: built
baseline_revision: 'f7c8fc881fae2b20cc04f3088f3eebd2bc4fd8fe'
route: oneshot
route_source: pinned
risk: low
review: quick
review_source: pinned
lenses_ran: [quick]
review_loop_iteration: 1
context: [_bmad-output/plan-e13-local-candidate61-packages.md]
---

<frozen-after-approval reason="human-owned intent">

## Intent

Windows、macOS 随包说明和正式交付说明都提供授权苹果 CMS V10 片源配置与切回演示目录的路径。明确 HTTPS、媒体格式、本地数据与授权边界。三份说明使用同一份配置指引，并继续纳入校验清单。

</frozen-after-approval>

## Implementation Notes

依据当前设置页面和 README 的授权 CMS 行为，复用同一份说明文本。验收时使用 .61 已存档的真实应用文件和隔离交付夹具。

## Verification

- 本地 Windows、macOS 说明与合成交付说明逐项比对，复算各说明及整个清单。
- 验证实际 .61 原始文件与样本交付的三项应用文件逐字节相同。正式组装脚本在合成来源信息下演练，真实签名、公证独立验收。
- 执行 Node 校验与交付回归。

## Results

- 旧版正式交付缺少 CMS 入口，新增回归在旧版失败。共用文本包含设置入口、测试和保存后重开、清除后演示目录、授权范围、本地数据及隐私说明。
- Windows 和 macOS 各生成本地和合成托管两份说明。五份产物（含组装说明）的 CMS 指引逐字一致，各自纳入清单。
- 隔离卷演练用 .61 的三项实际应用文件，组装后逐字节不变。原始存档的两份清单共 13 项 SHA-256 再次通过，合成交付 10 项 SHA-256 通过。Node 发行测试 17 项通过。

## Review

- 快审核对了窗口关闭行为与 GitHub Release 正文。已明确清除片源后要完全退出，再重新打开。隐私说明改为随附文件及发布下载区名称，适用于本地说明与发布正文。已扩大回归覆盖这两点。
