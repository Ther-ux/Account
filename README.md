# 个人记账本

一个在本机运行的 Web 记账本。支持录入收入和支出、查看最近记录，以及按日期、类别和金额区间统计收支。技术栈为 Go 标准库、原生 HTML/CSS/JS 和本地 JSON 文件。

## 当前进度

| 状态 | 内容 |
| --- | --- |
| 已完成 | 账目模型校验、JSON 持久化，以及新增、最近记录、收支统计三个真实 API |
| 已完成 | 记账表单、最近记录列表、组合筛选统计；保存后列表与统计按已应用条件自动刷新 |
| 已完成 | 三个区域的锚点导航、统一样式、手机布局、键盘焦点和加载／错误／空状态 |
| 已验证 | 真实浏览器录入与筛选、320～1440 像素布局、失败重试，以及重启后读取原账本 |
| 可选 | 核心功能联调通过后增加收支对比图 |

## 九人分工

开工前把姓名补到对应编号。每位成员独立实现本行的代码模块，在自己的分支提交并开 PR。

| 编号／姓名 | 模块 | 负责文件 | 验收标准 |
| --- | --- | --- | --- |
| 1／待填写 | 账目模型与校验 | `internal/ledger/record.go` | 定义 `Record`，校验真实日期、正金额、收支类型及类别是否匹配 |
| 2／待填写 | JSON 持久化 | `internal/store/json.go` | 首次读取返回空账本。新增记录后重启可读回，文件损坏时保留原文件，并发追加不会丢记录 |
| 3／待填写 | 新增账目 API 与服务接线 | `internal/httpapi/create.go`、`cmd/ledger/main.go` | 合法记录返回 `201` 并保存，非法请求返回 `400`。将 4、5 号的处理函数接到服务入口 |
| 4／待填写 | 最近记录 API | `internal/httpapi/recent.go` | 按录入顺序倒序返回，默认 10 笔，支持 `limit`，空账本返回 `[]` |
| 5／待填写 | 收支统计 API | `internal/httpapi/stats.go` | 能组合筛选日期、类别及金额，正确返回总收入、总支出和结余 |
| 6／待填写 | 记账表单 | `web/record.js` | 能填写日期、类型、金额、类别和备注，显示提交结果，并在保存后发出刷新事件 |
| 7／待填写 | 最近记录界面 | `web/recent.js` | 展示账目列表、空状态和失败提示，记账成功后自动刷新 |
| 8／待填写 | 统计筛选界面 | `web/stats.js` | 能设置及清空筛选条件，正确显示三项金额，记账成功后按当前条件刷新 |
| 9／待填写 | 页面框架与导航 | `web/index.html`、`web/app.css`、`web/app.js` | 完成导航和统一样式，接入三个前端模块，保证三个区域都能访问 |

## 本地运行

安装 Go 1.22 或更新版本，在仓库根目录运行：

```powershell
go run ./cmd/ledger
```

打开 [http://127.0.0.1:8080](http://127.0.0.1:8080)。如果端口已被占用，可以执行 `go run ./cmd/ledger -addr 127.0.0.1:8081`，然后访问 `8081` 端口。用 `Ctrl+C` 停止服务。

在仓库根目录启动，以便读取 `web/` 并将账本保存到 `data/records.json`。首次启动显示空账本；成功保存后列表与统计自动更新。`data/` 已加入 Git 忽略规则。

运行 `go test ./...` 和 `go vet ./...` 检查后端。完整联调结果、浏览器验收步骤及截图见 [任务 9 联调记录](docs/contributions/member09.md)。

## 并行开发约定

### 账目格式

```json
{
  "date": "2026-11-07",
  "type": "expense",
  "amountCents": 1250,
  "category": "饮食",
  "note": "午饭"
}
```

- `date` 使用 `YYYY-MM-DD`，`note` 可以省略或留空。Go 中 `amountCents` 对应 `AmountCents int64`，其余字段为 `string`，JSON 字段名以上面的示例为准。
- `amountCents` 使用正整数，单位是分。前端录入和显示用元，例如 `12.50` 元对应 `1250` 分。
- `income` 可用类别为工资、奖金、其他收入。`expense` 可用类别为饮食、交通、购物、其他支出。
- 账本存到 `data/records.json`，文件内容为账目数组，旧记录在前，新记录追加到末尾。仅存储模块负责直接读写文件。

### HTTP API

| 方法与路径 | 请求 | 成功结果 |
| --- | --- | --- |
| `POST /api/records` | JSON 请求体使用上面的账目格式 | `201`，返回保存后的账目对象 |
| `GET /api/records/recent` | 可选 `limit`，范围 `1`～`100`，默认 `10`，超出范围或格式无效返回 `400` | `200`，按录入顺序倒序返回账目数组。补记旧日期的账目也排在第一条 |
| `GET /api/stats` | 可选 `from`、`to`、`category`、`minCents`、`maxCents` | `200`，返回 `incomeCents`、`expenseCents`、`balanceCents` |

统计时同时应用所有给定条件，日期与金额区间都包含端点，日期格式与账目一致，金额条件使用非负整数分。无匹配时三项金额均为 `0`，结余等于收入减支出。输入不合法或区间上下限颠倒时返回 `400`。存储读写失败返回 `500`，错误响应使用 `{"error":"具体原因"}`。

### 模块如何接起来

- **模型与存储**由 1 号在 `ledger` 包提供 `Record` 和 `func Validate(record Record) error`，2 号在 `store` 包提供 `func NewJSONStore(path string) *JSONStore`，以及 `JSONStore` 的 `Append(record ledger.Record) error` 和 `List() ([]ledger.Record, error)` 方法。`List()` 按文件中的顺序返回。1 号可以先提交字段定义，再完成校验，方便其他后端成员尽早引用。
- **接口接线**由 3 号在 `main.go` 创建一个存储实例 `records`，用 `httpapi.Create(records.Append)`、`httpapi.Recent(records.List)`、`httpapi.Stats(records.List)` 替换三个占位处理函数。三个函数均返回 `http.HandlerFunc`。存储模块接入前，可以传入同签名的模拟函数开发 API。
- **前端**由 6、7、8 号分别导出 `initRecord(container)`、`initRecent(container)`、`initStats(container)`，由 9 号用 `<script type="module" src="/app.js"></script>` 加载 `app.js`，在其中接入 `#record-panel`、`#recent-panel`、`#stats-panel`。记账成功后在 `window` 发出 `ledger:changed` 事件，最近记录和统计模块各自监听并刷新。API 接通前可以用同样结构的模拟数据开发。

## GitHub 协作与联调

1. 首次克隆仓库，从 `main` 新建个人分支，例如 `feature/recent-records`。用自己的 Git 作者信息提交负责的文件，推送分支后开 PR，请另一位组员复审。
2. 优先合并模型和存储，再接通「录入 → 保存 → 最近记录」，随后联调统计。合并时保留各成员的原始提交，方便博客关联个人贡献。
3. 演示数据可用工资 `3000.00` 元、饮食 `12.50` 元、交通 `17.50` 元。验证总收入 `3000.00`、总支出 `30.00`、结余 `2970.00`。筛选交通应显示支出 `17.50` 元，重启服务后数据仍能查询。
4. 从已验收的提交发布 GitHub Release。提交前核查仓库、Release 和团队博客链接的访问权限。
