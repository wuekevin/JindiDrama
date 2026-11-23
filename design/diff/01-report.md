# 01 首页推荐 — 差异报告（G-UI-01）

- 参考：`design/reference/01-首页推荐.png`（1448×1086） 实际：`design/rendered/01-首页推荐.png`（Playwright Chromium，viewport 1448×1086，DPR 1）
- 差异图：`design/diff/01-diff.png`；响应式截图：`design/rendered/responsive/01-{1366x768,1920x1080,2560x1440}.png`
- 指标（`node frontend/tools/diff.mjs 01-首页推荐`）：像素不一致 ≈26%（几乎全部来自海报/人物图像差异），32px 块亮度 MAE ≈19（布局/明暗层次接近）。该指标仅辅助，结论以人工几何核对为准。

## 已对齐
侧栏 200px、导航项 52px/间距 57px、顶栏 56px（标签、搜索框 440px、铃铛/头像/窗口按钮）、Hero 1215×294 圆角 24、题材 chips、今日上新 6 卡（230px 高）、排行榜 5 列含金银铜徽标、继续观看 3 卡含进度条；各区块纵向基线与设计稿偏差 ≤6px。

## 已记录偏差
1. 海报/Hero 人物：原图为 AI 生成示例图，本项目用原创程序化 SVG 海报（渐变+散景+剪影）替代，避免版权问题；视觉细节必然不同。
2. 品牌名：设计稿“快剧短剧”，按 README 使用“金帝短剧”，Logo 为原创图形。
3. Hero 标题字体：设计稿为书法体，使用系统楷体回退（STKaiti/KaiTi），Windows 与 macOS 字形略有差异。
4. 侧栏图标使用 Lucide，与设计稿线条图标略有差异。
5. 首次截图后修正一轮：修复排行榜 `<ol>` 默认序号、Hero 人物比例、侧栏/顶栏标签位置、卡片高度（见 git 前后差异）。

## 验证记录（macOS arm64，Chromium 159，2026-11-22）
- `npx vitest run`：3 文件 10 用例通过
- `npx playwright test`：12 用例通过（结构、轮播、收藏、榜单、搜索跳转、导航、错误/空态、键盘、3 种分辨率无横向溢出）
- 状态：01 **Verified（UI/交互，Mock 数据）**
