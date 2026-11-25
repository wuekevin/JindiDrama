# DESIGN.md — 金帝短剧 Design System
Token 定义于 `frontend/src/styles/tokens.scss`。
- 色彩：背景 #0f0809/#170e10/#1e1214；强调 #ee4b39→#ff7a5c；金 #f4b740；文字 #f6ece9 / #c8b7b3 / #8f7d79；边框 rgba(255,255,255,.09)。
- 圆角：6/10/14/20/26/pill；卡片 14–16，Hero 24，主面板左上 26。
- 字体：PingFang SC / 微软雅黑；标题 22–25 粗体；正文 13–15；Hero 标题用楷体 display。
- 布局：侧栏 200；顶栏 56；主区内边距 14/20；参考画布 1448×1086，>1448 时 zoom 等比放大（≤2×），<1448 内容区滚动。
- 组件：BaseButton(primary/ghost/outline)、ProgressBar、EmptyState、ErrorState、Skeleton、Modal、Drawer、ToastHost、NewDramaCard、HeroCarousel。
- 图片：原创程序化 SVG 海报（mocks/posters.ts），不使用设计稿 PNG 作背景。
