# 开机小助手 · boot-tool

适用于 Windows 的开机时间统计小工具：**一眼看清"这次开机花了多久、开机后已经跑了多久"**。开机耗时取自 Windows 自身的启动日志打点（内核启动 → 登录完成），**开启"快速启动"的机器也不会算错**。

- **装好就不用管**：安装时默认勾选开机自启，每次登录 Windows 会在**右下角自动弹出卡片**，告诉你这次开机用了多久；约 30 秒自动消失，点一下就能看详情
- **口径可靠**：不拿程序自身启动时间凑数，也不拿"内核在线时长"冒充开机耗时——后者在快速启动下会跨关机累计，正是旧版"刚开机显示一个多小时"的原因
- 单文件 exe 双击即用：无控制台黑窗、无网络请求、不上传任何数据
- 也提供 `boot-tool-setup.exe` 安装包（开始菜单快捷方式 + 控制面板卸载项，**免管理员**）
- 零第三方依赖（Go 标准库 + 原生 Win32 API），内存占用极低
- 中英文自动切换（跟随 Windows 显示语言）
- 启动失败**一定可见**：错误对话框 + `%TEMP%\boot-tool.log`，另有 `boot-tool-debug.exe` 诊断版

## 功能（P0）

| 功能 | 说明 |
|---|---|
| 本次开机耗时 | 系统日志打点：`Kernel-Boot`/`Kernel-General`（启动）→ `Winlogon 7001`（登录完成）；秒级，不含 BIOS 自检 |
| 开机完成时刻 | 本次会话"桌面可用"的时间（登录完成那条日志），本地时区展示 |
| 已运行时长 | 自开机完成起实时刷新（每秒），格式如 `3天4小时5分6秒` / `3d 4h 5m 6s`，含睡眠/休眠 |
| 快速启动正确 | 混合启动（BootType=1）标注"（快速启动）"，并给出"上次冷启动 X 秒"作参照 |
| 睡眠恢复 | 睡眠/休眠唤醒后"已运行"连续推进；开机耗时/开机完成不受影响 |
| 开机自启 | 安装时默认开启（可在安装向导取消），写入 `HKCU\...\Run`，任务管理器"启动应用"可见可关；卸载自动清除 |
| 启动弹窗 | 自启启动时在右下角显示本次开机耗时，约 30 秒自动消失，点击进入主窗口；不抢焦点、不占任务栏 |
| 单实例 | 重复双击不会开出第二个窗口，而是激活已在运行的窗口 |

窗口底部有一行小字口径提示；时区切换、夏令时、NTP 校时都不会引起"已运行"跳变。

## 下载与使用

从 [Releases](../../releases) 下载（或按下方说明自行构建）：

| 资产 | 用途 |
|---|---|
| `boot-tool-setup.exe` | **安装包**：双击走完安装向导（默认勾选"开机时自动启动"），开始菜单出现「开机小助手」，控制面板可卸载；装到 `%LOCALAPPDATA%\Programs\BootTool`，**不需要管理员权限** |
| `boot-tool.exe` | 绿色单文件，解压/下载后双击即用，无需安装 |
| `boot-tool-debug.exe` | 控制台诊断版：逐步打印启动过程，打不开时用它定位（见下方「打不开？」） |
| `SHA256SUMS.txt` | 上述三个文件的 SHA256 校验值 |

### 日常使用（装好之后）

装完就不用管了：每次登录 Windows，约 3 秒后**右下角弹出卡片**，显示这次开机用了多久；约 30 秒自动消失。想细看就点卡片，或从开始菜单打开主窗口。

```
主窗口                                          右下角卡片
┌─ 开机小助手 ────────────────────────┐        ┌──────────────────────────────┐
│ 已运行：11分29秒                      │        │ 本次开机耗时（快速启动）        │
│ 本次开机耗时：3秒（快速启动）           │        │ 3秒                          │
│ 开机完成：2026-11-08 12:12:23         │        │ 开机完成：2026-11-08 12:12:23  │
│ 上次冷启动 13秒；耗时取自系统启动日志    │        │ 点击查看详情                   │
│ （内核启动→登录完成），不含 BIOS 自检；  │        └──────────────────────────────┘
│ 已运行含睡眠/休眠                      │
└─────────────────────────────────────┘
```

绿色版（不装安装包）也可以自己注册开机自启：

```powershell
boot-tool.exe --install-startup     # 设置开机自动启动（写 HKCU\...\Run，免管理员）
boot-tool.exe --uninstall-startup   # 取消开机自动启动
boot-tool.exe --autostart           # 只弹右下角卡片（自启用，一般不用手动敲）
boot-tool.exe --help                # 命令行说明
```

任务管理器 →「启动应用」里也能看到并禁用 `BootTool` 这一项；卸载安装包会自动清掉它。

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

产物为单个 `boot-tool.exe`（约 1.9 MB），拷到任意 Windows 10 1809+ / Windows 11 x64 机器即可运行，无需运行时、无需管理员权限。

**安装包**（需 [Inno Setup 7.x](https://jrsoftware.org/isdl.php)，Windows 上执行，先构建 `dist\boot-tool.exe`）：

```powershell
ISCC.exe /DMyAppVersion=0.4.0 packaging\boot-tool.iss   # 产物 dist\boot-tool-setup.exe
```

发布由 CI 完成：打 `vX.Y.Z` 标签即触发 `release.yml`，产出 `boot-tool.exe` + `boot-tool-debug.exe` + `boot-tool-setup.exe` + `SHA256SUMS.txt` 并挂到 GitHub Release；`ci.yml` 在 PR / `main` 推送上跑 vet、测试、交叉编译与安装包编译。

## 打不开？（故障排查）

**v0.2.0 起，任何启动失败都会弹出错误对话框**（不再无声退出），并写入日志 `%TEMP%\boot-tool.log`。按下面顺序排查：

1. **蓝色提示"Windows 已保护你的电脑"**（SmartScreen，未签名程序常见）：点 **"更多信息" → "仍要运行"**。
2. **文件被系统锁定**：右键 `boot-tool.exe` → **属性** → 勾选底部 **"解除锁定"** → 确定，再双击。
3. **确认下载完整**（GitHub 在部分地区下载可能中断）：命令行执行
   `certutil -hashfile boot-tool.exe SHA256`，与 Release 页 `SHA256SUMS.txt` 对比；不一致请重新下载。
4. **仍打不开**：运行 **`boot-tool-debug.exe`**（控制台诊断版），它会把启动过程逐步打印出来（`stage: ...` 逐行，最后一行即卡点）；把黑窗内容和 `%TEMP%\boot-tool.log` 一起反馈，即可定位。
5. **卡在 `stage: clock FAILED`**（v0.3.0 已修复）：日志出现 `stage: clock ok (source=...)` 说明时钟源已选中；若仍失败，说明该机器的 `kernelbase` / `kernel32` / `GetTickCount64` 三个入口都不可用，把日志发来即可。
6. 被杀毒软件隔离的话，请在 Windows 安全中心 → 病毒和威胁防护 → 保护历史记录中恢复/放行（程序无网络、无持久化，可放心放行）。
7. **安装包装不上**：`boot-tool-setup.exe` 为按用户安装（免管理员），装到 `%LOCALAPPDATA%\Programs\BootTool`；若企业策略拦截写入用户目录，会弹出可见错误对话框，按提示反馈即可。
8. **开机没有弹窗**：先确认任务管理器 →「启动应用」里的 `BootTool` 是"已启用"；再手动跑一次安装目录里的 `boot-tool.exe --autostart`，若卡片能出来说明自启项被禁用或被杀软拦截；用 `--install-startup` 可重新注册。
9. **数字看起来不对**（v0.4.0 起）：主窗口小字会写出口径。若显示"本次开机耗时：未知"，说明系统事件日志读不到（日志被清理/策略禁止），此时"已运行"是内核计时兜底值、快速启动后可能偏大；日志 `%TEMP%\boot-tool.log` 里的 `stage: boot-record ...` 一行能看出实际取到的启动/登录打点。
10. **窗口无响应 / 双击没反应**（v0.4.1 起）：
   - 双击时若已有实例在运行，程序会**激活它的窗口**；若 5 秒内找不到窗口，会弹出说明框（不再静默退出）。
   - 真出现无响应，`%TEMP%\boot-tool.log` 里会有 `watchdog: UI thread unresponsive for Ns` 这类记录，把它发来即可定位；同时可在任务管理器结束 `boot-tool.exe` 后重开。
   - 弹窗 30 秒后自动消失；想提前收掉点一下卡片进主窗口，或直接点关闭。

## 开发与测试

```bash
go test ./...                 # 单元测试（任意平台；Windows 上会额外读真实事件日志与时钟源）
go vet ./...                  # 静态检查
GOOS=windows go vet ./...     # Windows 侧静态检查
go run ./cmd/boot-tool        # 非 Windows：控制台冒烟模式（每秒打印快照）
go run ./cmd/boot-tool --autostart   # 打印一次卡片内容（模拟自启）
```

代码结构：

```
cmd/boot-tool/        入口（flag 解析 + 启动 stage 序列 + panic 兜底）
internal/clock/       时钟抽象（Windows: 分层解析 kernelbase/kernel32/GetTickCount64；Linux: /proc/uptime；测试: Fake）
internal/bootlog/     开机耗时事实源：读 System 日志打点（Kernel-Boot/Kernel-General → Winlogon 7001），含快速启动识别
internal/locale/      语言检测（GetUserDefaultUILanguage / LANG）
internal/tfmt/        时长与时间格式化（中/英）
internal/app/         展示装配（主窗口 View + 弹窗 ToastView）
internal/gui/         Win32 原生窗口与右下角弹窗（裸 syscall，无第三方 GUI 库）+ 单实例激活
internal/autostart/   开机自启注册（HKCU\...\Run 单值，免管理员）
internal/diag/        启动诊断：日志 %TEMP%\boot-tool.log + 错误/信息对话框（失败可见）
packaging/            Inno Setup 安装脚本 boot-tool.iss
.github/workflows/    ci.yml（PR 门禁）+ release.yml（打 tag 发三资产）
docs/                 PRD、技术方案、测试计划
```

## 口径说明（为什么数字对得上）

- **本次开机耗时**来自 Windows 自己的启动日志：`Kernel-Boot 27`（冷启动/快速启动标记）或 `Kernel-General 12`（冷启动）→ `Winlogon 7001`（登录完成）之差。事件查看器里能逐条对上，**不含 BIOS 自检**（固件阶段不写日志）。
- **为什么不用"内核在线时长"**：Windows 的"快速启动"把关机做成了休眠，内核会话不重建，`QueryInterruptTime`/`GetTickCount64`/`LastBootUpTime` 会跨关机连续累计——旧版因此把上一次会话的几小时算成"这次开机耗时"。内核计数现在只用于启动日志与极端兜底。
- **已运行时长**自"开机完成"起算：首次读数用墙钟差，之后用单调计数推进，所以 NTP 校时、手工改表都不会让它跳变，睡眠/休眠时间照常累计。
- **读不到日志时**：界面显示"本次开机耗时：未知"并提示回退风险，绝不编一个看起来合理的数字。
- 详细设计与取舍见 [`docs/design.md`](docs/design.md)。

## 文档

- [`docs/PRD.md`](docs/PRD.md) — 需求与验收标准（AC0–AC14）
- [`docs/design.md`](docs/design.md) — 技术方案（含 PRD 开放问题决策）
- [`docs/test-plan.md`](docs/test-plan.md) — 测试计划与验收回填表

## 路线图

- **P1**：纯运行时长（剔除睡眠）、累计开机时长 + 历史会话、托盘常驻
- **P2**：CSV 导出、超时提醒、更完整多语言

## License

[MIT](LICENSE)
