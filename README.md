# 金帝短剧：BMAD 全项目启动包

项目	最终命名
中文名称	金帝短剧
英文名称	Jindi Short Drama（暂定）
项目代号	jindi-drama
Windows 程序	JindiDrama.exe
安装程序	JindiDrama-Setup.exe
macOS 应用	金帝短剧.app
本地数据目录	JindiDrama
Go Module	jindi-drama（暂定）
UI 标题	金帝短剧

这是 **BMAD 开发任务与设计素材交付包**，不是已完成、已打通红果 API 的商业客户端。

## 第一件事
1. 将本包放到新项目的根目录（也可直接把本包作为项目根目录）。
2. 确认 `design/reference/` 有 `01` 到 `10` 共 10 张 PNG。
3. 在本目录打开 Claude Code；如果你已安装 BMAD 插件，优先使用现有插件，通过 `/bmad` 或 `/bmad-help` 查看工作流。**不要重复安装或覆盖已有插件配置。**
4. 将 `bmad/MASTER-PROMPT.md` 全文发送给 Claude Code 的 BMAD Master / 项目协调 Agent。
5. 必须先完成第 0 阶段视觉设计系统和 **01 首页** 的截图验收，再进入其余九页；不要一口气生成全部页面。

## 文档
- `bmad/MASTER-PROMPT.md`：完整交付式任务书（直接发送给 BMAD）
- `docs/PRD.md`：范围、功能、验收、边界
- `docs/ARCHITECTURE.md`：技术栈、模块设计、数据模型、接口适配
- `docs/ROADMAP-AND-STORIES.md`：阶段、Epic、Story 与验收定义
- `docs/UI-ACCEPTANCE.md`：十页逐页视觉/交互清单
- `docs/API-LEGAL-GATES.md`：红果 API 可接入性与授权门槛
- `contracts/provider-contracts.ts`：前端 Provider 类型定义草案
- `contracts/api-capabilities.json`：功能门禁与降级状态
- `reference-starter/旧版演示工程-仅供参考.zip`：之前生成的 Wails 演示工程，不作为已验证产品代码

## 特别注意
PNG 是概念设计效果图，影视海报、人物、数据均为占位；不要用完整图片当网页背景。功能实现需使用原创资产或具有合适授权的素材。不得声称非公开红果 API 已得到授权、可稳定调用或已经通过端到端测试。

## 开发与验证（P0 已实现）
```
cd frontend && npm install
npm run dev            # 浏览器预览 http://localhost:5173
npm run typecheck && npm run test && npm run build && npm run test:e2e
cd .. && go test ./...  # 需先 npm run build 生成 frontend/dist（main.go 内嵌）
wails build -s          # 本机原生构建（macOS 已验证可编译；Windows 未验证）
```
状态见 `docs/STATUS.md`；十页截图 `design/rendered/`，差异报告 `design/diff/`。
