# 任务 9：页面框架与导航

本次将本地已有的任务 9 代码上传到 `Ther-ux/Account` 的 `feature/page-shell` 分支。姓名、学号和独立开发经历由本人据实补充；本文仅记录本次上传范围和实际检查结果。

## 提交文件

- `web/index.html`：中文首页、三个锚点导航、跳转到主要内容链接，以及 `record-panel`、`recent-panel`、`stats-panel` 三个模块容器。
- `web/app.css`：页面布局、表单、按钮、记录表格、统计卡片和加载、错误、空状态样式，以及窄屏和减少动画的适配规则。
- `web/app.js`：导入三个业务模块并传入对应容器，通过 `WeakSet` 避免重复初始化，更新导航的 `aria-current`，在模块加载失败时显示提示。
- `docs/contributions/member09.md`：本次上传与交接说明。

三个代码文件沿用本地 `成员09_页面框架与导航_代码/web/` 中的现有内容。

## 实际检查

检查日期：2026-11-07（北京时间）。

- 执行 `node --check web/app.js`：通过。
- 使用 Node 静态断言核对三个 section ID、三个容器 ID 均唯一，三个导航锚点存在，页面加载 `/app.css` 和模块入口 `/app.js`：通过。
- 已读取主分支 `d6a6b2a704ed6090e89a47b879bedea41256e7ee` 的文件树及 `web/recent.js`：该模块导出 `initRecent(container)`，其主要 class 与本次样式对应。

以上为语法和静态检查，尚未执行真实浏览器的桌面、窄屏、键盘操作或保存后自动刷新验收。

## 依赖与交接

核查时主分支已有 `web/recent.js`，尚缺 `web/record.js` 和 `web/stats.js`。本分支保留现有代码对真实模块的导入，不加入替身模块或模拟业务数据。两个文件补齐之前，记账和统计面板会显示模块加载失败提示，浏览器也会产生对应的资源加载错误，因此本次作为草稿 PR 上传，等待依赖齐备后联调。

- 6 号提供 `web/record.js` 的 `initRecord(container)`，保存成功后在 `window` 发出 `ledger:changed`。
- 7 号保持 `web/recent.js` 的 `initRecent(container)` 和事件刷新约定。
- 8 号提供 `web/stats.js` 的 `initStats(container)`，按当前条件监听刷新。
- 服务接线由 3 号负责，本次没有修改 `cmd/ledger/main.go`。

后续验收需确认三个脚本均成功加载、三个导航可达、表单保存后最近记录和统计更新，以及窄屏和键盘可操作。验收结果应据实补充到本文和 PR。
