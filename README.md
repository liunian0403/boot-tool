# 开机小助手 · boot-tool

适用于 Windows 的开机时间统计小工具：**一眼看清"什么时候开的机、开了多久"**，数值与 Windows 任务管理器"正常运行时间"同口径（含睡眠/休眠），精确到秒、永不漂移。

- 单文件 exe，双击即用：无安装、无控制台黑窗、无网络请求、不上传任何数据
- 零第三方依赖（Go 标准库 + 原生 Win32 API），内存占用极低
- 中英文自动切换（跟随 Windows 显示语言）

## 功能（P0）

| 功能 | 说明 |
|---|---|
| 开机时间 | 本次开机的绝对时刻（本地时区） |
| 已开机时长 | 实时刷新（每秒），格式如 `3天4小时5分6秒` / `3d 4h 5m 6s` |
| 口径对齐 | 含睡眠/休眠时间，与任务管理器"正常运行时间"一致 |
| 睡眠恢复 | 睡眠/休眠唤醒后数值连续，不重置不跳变 |

窗口底部有一行小字口径提示；时区切换、夏令时、NTP 校时都不会引起已开机时长跳变。

## 下载与使用

从 [Releases](../../releases) 下载 `boot-tool.exe`（或按下方说明自行构建），双击运行即可。

界面示意：

```
┌─ 开机小助手 ─────────────────────────────┐
│  已开机：3天4小时5分6秒                    │
│  开机时间：2026-11-05 13:10:26            │
│  含睡眠/休眠时间，与 Windows 任务管理器口径一致 │
└──────────────────────────────────────────┘
```

## 自行构建

需要 [Go](https://go.dev/dl/) 1.27+（任意平台均可交叉编译）：

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags "-s -w -H=windowsgui" -o dist/boot-tool.exe ./cmd/boot-tool
```

产物为单个 `boot-tool.exe`（约 1.4 MB），拷到任意 Windows 10 1809+ / Windows 11 x64 机器即可运行，无需运行时、无需管理员权限。

## 开发与测试

```bash
go test ./...                 # 单元测试（任意平台）
go vet ./...                  # 静态检查
GOOS=windows go vet ./...     # Windows 侧静态检查
go run ./cmd/boot-tool        # 非 Windows：控制台冒烟模式（每秒打印快照）
```

代码结构：

```
cmd/boot-tool/        入口
internal/clock/       时钟抽象（Windows: QueryInterruptTime；Linux: /proc/uptime；测试: Fake）
internal/uptime/      核心口径：开机时刻 = 现在 − 运行时长（含睡眠）
internal/locale/      语言检测（GetUserDefaultUILanguage / LANG）
internal/tfmt/        时长与时间格式化（中/英）
internal/app/         展示装配
internal/gui/         Win32 原生窗口（裸 syscall，无第三方 GUI 库）
docs/                 PRD、技术方案、测试计划
```

## 口径说明（为什么数字对得上）

- **已开机时长**来自 `QueryInterruptTime`：内核单调计数、**包含睡眠/休眠**，不受墙钟调整/时区/夏令时影响 —— 与任务管理器"正常运行时间"同语义，7 天不漂移。
- **开机时间** = 当前墙钟 − 已开机时长，每次刷新实时推导，不落盘、不依赖 WMI。
- 详细设计与取舍见 [`docs/design.md`](docs/design.md)。

## 文档

- [`docs/PRD.md`](docs/PRD.md) — 需求与验收标准（AC1–AC7）
- [`docs/design.md`](docs/design.md) — 技术方案（含 PRD 开放问题决策）
- [`docs/test-plan.md`](docs/test-plan.md) — 测试计划与验收回填表

## 路线图

- **P1**：纯运行时长（剔除睡眠）、累计开机时长 + 历史会话、托盘常驻 + 开机自启
- **P2**：CSV 导出、超时提醒、更完整多语言

## License

[MIT](LICENSE)
