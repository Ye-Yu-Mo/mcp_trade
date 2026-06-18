# Changelog

变更记录

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)

版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)

---

## 版本号说明

- **主版本号（Major）**：不兼容的 API 变更或架构重构
- **次版本号（Minor）**：向后兼容的功能新增（新模块、新页面、新接口）
- **修订号（Patch）**：向后兼容的问题修正、小优化、文档更新

---

## v0.2.0 — 2026-06-18

### Added

**价格提醒系统**
- `market.alerts` MCP 工具：设价格提醒、查活跃/已触发提醒、删除提醒
- 内存自动检测：每 3 秒对比缓存价格，触发时自动标记

**市场扫描器**
- `market.scanner` MCP 工具：一次调用扫全市场，按 24h 成交量排序

**合约情绪指标**
- `market.funding` MCP 工具：资金费率 + 多空拥挤度判断
- `market.oi` MCP 工具：未平仓合约 + 趋势强度信号

**订单改进**
- `order.place` 支持 `reduce_only` 参数（平仓/减仓）
- 仓位上限从 5% 提高到 50%

**体验优化**
- `market.klines` 支持逗号分隔多币种
- `market.watch` 多周期 K 线（1h / 4h / 1d）
- `trade.journal_list` 支持按 entry_type / tags 筛选
- `market.calendar` 经济日历

### Changed
- CI 修复：`npm ci` → `npm install`
- Go 测试 `-short` 跳过高频集成测试
- README 架构图改为 Mermaid

### Summary
| Metric | Value |
|--------|-------|
| MCP Tools | 18 (+4) |
| Go Tests | 62 (+9) |

---

## v0.1.0 — 2026-06-18

Initial Release — AI 主观交易系统首个可用版本。

### Added

**M1: Trading Server 骨架**
- Go 项目结构 + Binance Futures REST API 连接
- 行情端点：klines, ticker, orderbook
- 账户端点：balance, positions
- DuckDB 持久化交易日志
- HTTP API Token 鉴权 + Bearer 中间件

**M2: MCP Server + 核心工具**
- TypeScript MCP Server（@modelcontextprotocol/sdk）
- 6 个 MCP 工具：market.klines, market.ticker, market.price, market.orderbook, account.balance, account.positions
- 双通道输出：content (Markdown) + structuredContent (JSON)

**M3: 订单管理 + 风控闸门**
- 下单/撤单/查单（LIMIT, MARKET, STOP_MARKET）
- Plan/Apply 闸门——先预览后执行，plan_id 防篡改
- RiskManager：仓位上限、止损上限、每日亏损上限
- 价格精度/数量精度/最小名义价值校验
- 币安 API 限频器（250ms 签名请求间隔）
- 4 个 MCP 订单工具：order.place, order.cancel, order.list, order.status

**M4: 交易日志 + 经验系统**
- trades 表自动记录（入场理由、行情快照、盈亏）
- 风控集成到 Apply 阶段（实时查余额 + 日亏损）
- 3 个 MCP 工具：trade.history, trade.journal, trade.performance
- 绩效统计：胜率、盈亏比、最大回撤、Profit Factor

**M5: 实时行情 + WebSocket**
- 内存行情缓存（sync.RWMutex 并发安全）
- 币安 WebSocket 行情流（bookTicker + kline + depth）
- User Data Stream（ORDER_TRADE_UPDATE 自动更新 trades）
- REST 轮询 fallback（testnet WS 不可用时）
- market.watch MCP 工具（一键全币种快照）

**M6: 交易看板**
- React + Tailwind CSS v4 暗色主题
- 4 个状态卡片：总资产、未实现盈亏、今日已实现、保证金水位
- 持仓/挂单表格 + SVG 资金曲线 + 风控进度条
- 5s 自动刷新 + Trading Server 一体化托管

**M7: 生产就绪**
- systemd 服务（开机自启 + 崩溃重启 + 优雅关闭）
- 主网部署（WebSocket 行情 + User Data Stream 双通道在线）
- 错误码标准化（codes.go 18 个常量）
- 健康检查增强（WS 状态 + 缓存项数 + uptime）
- README 文档

### Summary

| Metric | Value |
|--------|-------|
| MCP Tools | 14 (market×5, account×2, order×4, trade×3) |
| API Endpoints | 16 |
| Go Tests | 53 |
| Lines of Code | Go ~2000, TypeScript ~800, React ~600 |
| Stack | Go + Chi + DuckDB, TypeScript + MCP SDK, React + Tailwind |
