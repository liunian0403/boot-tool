# 开机小助手 · boot-tool

适用于 Windows 的开机时间统计小工具：**一眼看清"什么时候开的机、开了多久"**，数值与 Windows 任务管理器"正常运行时间"同口径（含睡眠/休眠），精确到秒、永不漂移。

- 单文件 exe 双击即用：无控制台黑窗、无网络请求、不上传任何数据
- 也提供 `setup.exe` 安装包（开始菜单快捷方式 + 控制面板卸载项，**免管理员**）
- 零第三方依赖（Go 标准库 + 原生 Win32 API），内存占用极低
- 中英文自动切换（跟随 Windows 显示语言）
- 启动失败**一定可见**：错误对话框 + `%TEMP%\boot-tool.log`，另有 `boot-tool-debug.exe` 诊断版

## 功能（P0）

| 功能 | 说明 |
|---|---|
| 开机时间 | 本次开机的绝对时刻（本地时区） |
| 已开机时长 | 实时刷新（每秒），格式如 `3天4小时5分6秒` / `3d 4h 5m 6s` |
| 口径对齐 | 含睡眠/休眠时间，与任务管理器"正常运行时间"一致 |
| 睡眠恢复 | 睡眠/休眠唤醒后数值连续，不重置不跳变 |

窗口底部有一行小字口径提示；时区切换、夏令时、NTP 校时都不会引起已开机时长跳变。

## 下载与使用

从 [Releases](../../releases) 下载（或按下方说明自行构建）：

| 资产 | 用途 |
|---|---|
| `setup.exe` | **安装包**：双击走完安装向导，开始菜单出现「开机小助手」，控制面板可卸载；装到 `%LOCALAPPDATA%\Programs\BootTool`，**不需要管理员权限** |
| `boot-tool.exe` | 绿色单文件，解压/下载后双击即用，无需安装 |
| `boot-tool-debug.exe` | 控制台诊断版：逐步打印启动过程，打不开时用它定位（见下方「打不开？」） |
| `SHA256SUMS.txt` | 上述三个文件的 SHA256 校验值 |

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
# 绿色单文件（无黑窗）
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags "-s -w -H=windowsgui" -o dist/boot-tool.exe ./cmd/boot-tool

# 诊断版（控制台，逐步打印启动 stage）
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags "-s -w" -o dist/boot-tool-debug.exe ./cmd/boot-tool
```

产物为单个 `boot-tool.exe`（约 1.8 MB），拷到任意 Windows 10 1809+ / Windows 11 x64 机器即可运行，无需运行时、无需管理员权限。

**安装包**（需 [Inno Setup 7.x](https://jrsoftware.org/isdl.php)，Windows 上执行，先构建 `dist\boot-tool.exe`）：

```powershell
ISCC.exe /DMyAppVersion=0.2.0 packaging\boot-tool.iss   # 产物 dist\setup.exe
```

发布由 CI 完成：打 `vX.Y.Z` 标签即触发 `release.yml`，产出 `boot-tool.exe` + `boot-tool-debug.exe` + `setup.exe` + `SHA256SUMS.txt` 并挂到 GitHub Release；`ci.yml` 在 PR / `main` 推送上跑 vet、测试、交叉编译与安装包编译。

## 打不开？（故障排查）

**v0.2.0 起，任何启动失败都会弹出错误对话框**（不再无声退出），并写入日志 `%TEMP%\boot-tool.log`。按下面顺序排查：

1. **蓝色提示"Windows 已保护你的电脑"**（SmartScreen，未签名程序常见）：点 **"更多信息" → "仍要运行"**。
2. **文件被系统锁定**：右键 `boot-tool.exe` → **属性** → 勾选底部 **"解除锁定"** → 确定，再双击。
3. **确认下载完整**（GitHub 在部分地区下载可能中断）：命令行执行
   `certutil -hashfile boot-tool.exe SHA256`，与 Release 页 `SHA256SUMS.txt` 对比；不一致请重新下载。
4. **仍打不开**：运行 **`boot-tool-debug.exe`**（控制台诊断版），它会把启动过程逐步打印出来（`stage: ...` 逐行，最后一行即卡点）；把黑窗内容和 `%TEMP%\boot-tool.log` 一起反馈，即可定位。
5. 被杀毒软件隔离的话，请在 Windows 安全中心 → 病毒和威胁防护 → 保护历史记录中恢复/放行（程序无网络、无持久化，可放心放行）。
6. **安装包装不上**：`setup.exe` 为按用户安装（免管理员），装到 `%LOCALAPPDATA%\Programs\BootTool`；若企业策略拦截写入用户目录，会弹出可见错误对话框，按提示反馈即可。

## 开发与测试

```bash
go test ./...                 # 单元测试（任意平台）
go vet ./...                  # 静态检查
GOOS=windows go vet ./...     # Windows 侧静态检查
go run ./cmd/boot-tool        # 非 Windows：控制台冒烟模式（每秒打印快照）
```

代码结构：

```
cmd/boot-tool/        入口（启动 stage 序列 + panic 兜底）
internal/clock/       时钟抽象（Windows: QueryInterruptTime；Linux: /proc/uptime；测试: Fake）
internal/uptime/      核心口径：开机时刻 = 现在 − 运行时长（含睡眠）
internal/locale/      语言检测（GetUserDefaultUILanguage / LANG）
internal/tfmt/        时长与时间格式化（中/英）
internal/app/         展示装配
internal/gui/         Win32 原生窗口（裸 syscall，无第三方 GUI 库）
internal/diag/        启动诊断：日志 %TEMP%\boot-tool.log + 错误对话框（失败可见）
packaging/            Inno Setup 安装脚本 boot-tool.iss
.github/workflows/    ci.yml（PR 门禁）+ release.yml（打 tag 发三资产）
docs/                 PRD、技术方案、测试计划
```

## 口径说明（为什么数字对得上）

- **已开机时长**来自 `QueryInterruptTime`：内核单调计数、**包含睡眠/休眠**，不受墙钟调整/时区/夏令时影响 —— 与任务管理器"正常运行时间"同语义，7 天不漂移。
- **开机时间** = 当前墙钟 − 已开机时长，每次刷新实时推导，不落盘、不依赖 WMI。
- 详细设计与取舍见 [`docs/design.md`](docs/design.md)。

## 文档

- [`docs/PRD.md`](docs/PRD.md) — 需求与验收标准（AC0–AC8）
- [`docs/design.md`](docs/design.md) — 技术方案（含 PRD 开放问题决策）
- [`docs/test-plan.md`](docs/test-plan.md) — 测试计划与验收回填表

## 路线图

- **P1**：纯运行时长（剔除睡眠）、累计开机时长 + 历史会话、托盘常驻 + 开机自启
- **P2**：CSV 导出、超时提醒、更完整多语言

## License

[MIT](LICENSE)
