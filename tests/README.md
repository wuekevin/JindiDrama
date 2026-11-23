# 测试策略（交给 BMAD QA 实施）

`npm run test`：组件测试，路由、状态、筛选、选择剧集、评论弹幕 Mock。

`npm run test:e2e`：Playwright 逐页导航、交互、空态、错误与截图。

固定 `viewport: {width:1448,height:1086}`, `deviceScaleFactor:1`，等待字体与主要图片加载完成，再截屏；视频区域截图使用固定授权测试帧，避免随机差异。

图像差异需要人工核对几何和文字；照片不同不直接判 UI 失败，记录图像差异。所有测试须写明真实运行系统、时间和结果。
