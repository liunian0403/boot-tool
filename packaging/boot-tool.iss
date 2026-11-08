; 开机小助手 boot-tool · Inno Setup 7.1.1 安装脚本
; 契约见 docs/design.md §4.1（ADR-1）：按用户安装到 %LOCALAPPDATA%、免管理员（PrivilegesRequired=lowest）。
;
; 本地/CI 编译：
;   ISCC.exe /DMyAppVersion=0.2.0 packaging\boot-tool.iss     ; 版本由 CI 从 v* tag 传入
; 产物：..\dist\setup.exe（与 boot-tool.exe 同一构建产物，见 docs/design.md §4.3）
;
; 本文件必须保存为 UTF-8 with BOM：ISCC 只靠 BOM 识别 UTF-8，
; 否则中文在非中文系统（CI runner）上会按 ANSI 解析导致乱码或编译失败。

#ifndef MyAppVersion
  #define MyAppVersion "0.2.0"
#endif

[Setup]
; 固定 AppId：升级时覆盖同一安装位、卸载项唯一（首版即锁定，勿随意更改）
AppId={{A7C4E9D2-51B8-4F3A-9C61-2D8E44B7F013}
AppName=开机小助手
AppVersion={#MyAppVersion}
AppPublisher=boot-tool
DefaultDirName={localappdata}\Programs\BootTool
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputBaseFilename=setup
OutputDir=..\dist
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; 仅64 位（PRD §6：Windows 10 1809+ / Windows 11 x64）
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0.17763
UninstallDisplayName=开机小助手
UninstallDisplayIcon={app}\boot-tool.exe

[Languages]
; 安装器语言跟随系统（docs/design.md §9 AC7）
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "..\dist\boot-tool.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\开机小助手"; Filename: "{app}\boot-tool.exe"
Name: "{autodesktop}\开机小助手"; Filename: "{app}\boot-tool.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\boot-tool.exe"; Description: "{cm:LaunchProgram,开机小助手}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; 程序本身不向 {app} 写入任何数据（docs/design.md §7），这里只兜底清理可能残留的日志
Type: files; Name: "{app}\*.log"
