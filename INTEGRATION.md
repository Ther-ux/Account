# 整合记录

基础：用户提供的成员 01～09 提交包，目录前缀 member01/member02/member03 已展开到统一仓库结构。原始包不被修改，本项目在独立目录完成整合。

## 保留的公共契约

module personal-ledger；ledger.Record 五个字段；ledger.Validate；NewJSONStore、Append、List；Create、Recent、Stats；POST /api/records、GET /api/records/recent、GET /api/stats；initRecord/initRecent/initStats 与 ledger:changed 事件；三个原有容器 ID。

## 实际修正

- 以新提供的成员 01 模型替换临时参考模型；补充拒绝公历 0000 年。
- 补齐主程序、静态资源服务及共享存储实例接线。
- 存储在追加和读回时执行模型校验，避免结构合法但业务无效的 JSON 被当成正常账本。
- 新增 API 限制请求大小、拒绝未知字段及非预期方法；保存错误不暴露绝对路径。原保存错误测试随最终行为修正，并补上回归测试。
- recent 的 limit 统一为十进制数字，拒绝符号和重复参数。
- Stats 与 Insights 复用筛选和整数汇总，保留原统计响应。
- 额外引入 ledger:stats-updated/ledger:stats-error 供洞察联动，原有 ledger:changed 保持兼容。
- 添加 JSON/CSV 导出、预算偏好、嵌入资源和独立演示入口。
- 保留原有测试并补充输入、持久化、洞察、导出和主程序联调测试。

原始 ZIP 来源和最终文件 SHA256 在 SOURCE_MANIFEST.json。该记录描述代码来源与整合修改，不代替任何成员的 Git 提交历史或独立完成证明。
