# 技术方案：Windows 开机小助手（boot-tool）

> 依据 `docs/PRD.md`（v1，Gia 已复核）。本文档是实现与测试的唯一技术基准。

## 1. 总体架构

单进程、零第三方依赖的 Go 程序，交叉编译为**单文件 Windows exe**（无控制台窗口、无运行时依赖、无需安装）。

```
cmd/boot-tool/main.go        入口：装配 clock / locale / gui
internal/clock/              时钟抽象与平台实现（含测试 Fake）
internal/uptime/             核心口径逻辑：开机时刻 = 现在 − 运行时长
internal/locale/             语言检测（Windows UI 语言 / LANG 环境变量）
internal/tfmt/               时长与时间的展示格式化（中/英）
internal/app/                展示装配：Clock → View{标题, 开机时间, 已开机, 口径提示}
internal/gui/                Windows 原生窗口（user32 裸 API）；非 Windows 控制台模式
docs/                        PRD、本设计、测试计划
```

平台差异全部收敛在 `internal/clock` 与 `internal/gui` 两个包内，通过 build tag 划分；核心口径逻辑（`uptime`、`tfmt`、`app`）平台无关，在 Linux 上可直接单元测试。

## 2. 时钟口径设计（对应 PRD §2 四条硬性要求）

### 2.1 数据源

| 数据 | API | 说明 |
|---|---|---|
| 运行时长（含睡眠） | `kernel32!QueryInterruptTime` | 返回自系统启动以来的 100ns 单调计数，**包含睡眠/休眠时间**，不受墙钟调整、时区、夏令时影响 |
| 墙钟 | `time.Now()` | 仅用于展示与推导开机时刻的绝对值 |
| 开机时刻（推导） | `Boot = Now − Uptime` | 每次刷新实时推导，不单独存储 |

不把 WMI `Win32_OperatingSystem.LastBootUpTime` 作为运行时数据源：WMI 查询延迟高（数百 ms 到秒级，威胁 AC6 的 3 秒出数）、依赖 COM/权限、且其值本身也是由运行时长推导。WMI/`systeminfo` 仅作为**人工对账基准**（见 `docs/test-plan.md` AC1）。

### 2.2 四条硬性要求逐条实现方式

1. **与任务管理器"正常运行时间"同口径（含睡眠）**：`QueryInterruptTime` 计数包含 suspend 时间，语义即"含睡眠的正常运行时间"；启动后与系统权威来源对账（test-plan AC1/AC3）。
2. **睡眠/唤醒后连续不跳变**：运行时长来自单调计数，睡眠期间计数照常推进，唤醒后从同一计数续算——无重置路径、无累加状态可丢。窗口 `WM_TIMER` 每秒刷新，唤醒后下一个 tick 即显示连续值。
3. **持久化原子性（P1 范围）**：本期无持久化数据（见 §8 决策 2），该条随 P1"累计开机时长"实现：写临时文件 + `os.Rename`（NTFS 同卷原子替换）+ 校验和，断电自愈。本期代码不产生违反该条的状态。
4. **时区/夏令时不跳变**：内部一律基于 `time.Time` 的绝对时刻与单调运行时长计算；`Boot = Now − Uptime` 与 `Now` 使用同一时区，时区切换只改变 `t.Format` 的呈现，不改变任何计算。NTP 墙钟步进只影响 `Boot` 的绝对展示（与系统权威来源同步漂移），**运行时长完全不受影响**。

### 2.3 精度分析（AC1/AC2）

- 运行时长：`QueryInterruptTime` 分辨率 100ns，无累积误差 → 7 天无漂移（AC2）。
- 开机时刻 = 墙钟 − 运行时长：墙钟受 NTP 校准时，推导出的开机时刻与系统自身推导（`systeminfo`/任务管理器同样由运行时长推导）同向移动，保持 ≤1s 对齐；时区/夏令时切换不产生任何位移。
- 首屏（AC6）：窗口创建前先读一次快照，首帧即带数据；Go 程序启动到出窗 <300ms 量级，远优于 3s。

## 3. 平台实现

### 3.1 Windows（`GOOS=windows`，纯 syscall，CGO_ENABLED=0）

- **GUI**：直接调用 `user32`（`RegisterClassExW`/`CreateWindowExW`/`SetTimer`/消息循环），不引入第三方 GUI 库。
  - 窗口：`WS_OVERLAPPEDWINDOW`，标题"开机小助手"/"Boot Assistant"，约 460×200，系统默认位置。
  - 控件：两行 `STATIC` —— 大号粗体"已开机：3天4小时5分6秒"、常规"开机时间：2026-11-08 09:15:32"；底部灰色小字口径提示"含睡眠/休眠时间，与 Windows 任务管理器口径一致"。
  - 刷新：`SetTimer` 1s + `WM_TIMER` 重算；另处理 `WM_TIMECHANGE`（时区变化立即刷新）、`WM_DESTROY`（清理退出）。
  - 字体：`CreateFontW("Segoe UI")`，Windows 10/11 默认可用。
- **运行时长**：`QueryInterruptTime`（Win8+ 可用，覆盖 PRD 目标 Win10 1809+）。
- **语言**：`GetUserDefaultUILanguage` 主语言 ID = 0x04（中文）→ 中文界面，否则英文（AC7）。
- **构建**：`go build -ldflags "-s -w -H=windowsgui"` → 无控制台闪窗、无符号，产物 `boot-tool.exe`（预计 <3MB）。

### 3.2 非 Windows（开发/CI 用）

控制台模式，每秒打印一行快照（运行时长读 `/proc/uptime`，Linux 上含 suspend），用于本仓开发期的口径冒烟验证；不作为交付物。

## 4. 展示格式（对应 PRD §7 决策 3）

- 时长：中文 `3天4小时5分6秒`、英文 `3d 4h 5m 6s`；**从天到秒顺序、去掉尾部为 0 的单位、保留中间的 0**（`3天0小时4分11秒`），全 0 显示 `0秒`/`0s`。秒级精度常显（AC2）。
- 开机时刻：本地时区 `2006-01-02 15:04:05`，中英文一致（数字格式无歧义）。

## 5. 测试策略

| 层次 | 内容 | 运行环境 |
|---|---|---|
| 单元测试 | 口径逻辑：7 天无漂移、睡眠连续、NTP 步进、时区切换、负值钳制；格式化中/英全用例；App 装配 | Linux `go test ./...` |
| 编译期验证 | `GOOS=windows go vet ./...` + 交叉编译 exe | Linux |
| 冒烟 | 非 Windows 控制台模式实跑，与 `uptime -s` 对账 | Linux |
| Windows 手工验收 | AC1–AC7 逐项步骤 | 真机，见 `docs/test-plan.md` |

Windows 专属代码（win32、QueryInterruptTime）在 Linux 上无法运行时验证，故：能单测的逻辑全部下沉到平台无关包；剩余风险集中在 win32 调用层，由 test-plan 的真机步骤覆盖。

## 6. 交付物

- 源码 + 单元测试（本仓库）
- `README.md`（使用、构建、口径说明）
- `docs/`：PRD、设计、测试计划
- 构建说明：`GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -H=windowsgui" -o dist/boot-tool.exe ./cmd/boot-tool`

## 7. 风险与假设

1. `QueryInterruptTime` 为 Windows 8+ API —— 目标系统 Win10 1809+，满足；若实测发现个别环境不可用，回退 `GetTickCount64`（同为单调、含睡眠语义需实测确认）。
2. AC6 资源指标为 PRD 假设值：本方案无后台轮询之外的开销（1 次/秒的轻量重算），预期远低于阈值，真机实测后校准基线。
3. win32 裸调用层（struct 布局、消息循环）依赖 `syscall` 标准库，Go 版本升级需回归 `GOOS=windows` 编译。

## 8. PRD §7 开放问题决策（Gia 拍板）

| # | 问题 | 决策 |
|---|---|---|
| 1 | 交付形态 | **本期：单文件 exe，双击即用**；托盘常驻 + 开机自启顺延 P1（不影响 P0 验收，安装版无必要） |
| 2 | 历史会话记录与导出 | **本期砍掉，归 P1**；P0 无持久化，AC4（断电不损坏累计数据）随 P1 实现时验收，本期标注 N/A。原子写方案已在 §2.2 预留 |
| 3 | 展示格式 | **`3天4小时5分6秒`（天→秒、秒级常显）**，英文 `3d 4h 5m 6s`；不用 `72:04:11`（面向普通用户不友好，见 §4） |
| 4 | AC6 资源指标 | **保留为参考目标，不作阻塞验收项**；功能验收以 AC1/2/3/5/7 为准，AC6 在真机测试时实测并把数据记录进 test-plan 回填基线 |

对 PRD §2 的一处口径修正：开机时刻不采用 WMI 实时查询（理由见 §2.1），改为单调运行时长推导，权威来源仅作对账基准——该修正不改变任何验收标准。
