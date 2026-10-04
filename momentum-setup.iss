; Momentum Installer Script for Inno Setup
; Professional Windows installer with modern UI

#define MyAppName "Momentum"
#define MyAppVersion "2.0.1"
#define MyAppPublisher "Momentum Labs"
#define MyAppURL "https://github.com/HarshalPatel1972/momentum"
#define MyAppExeName "Momentum.exe"

[Setup]
; App Information
AppId={{B8F9D2E1-4A3C-4F5E-9B7D-2C8A1E6F4D3B}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
AppUpdatesURL={#MyAppURL}/releases

; Install Configuration
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
LicenseFile=LICENSE
OutputDir=installer-output
OutputBaseFilename=MomentumSetup
SetupIconFile=bridge-ui\build\windows\icon.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern

; Platform
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=lowest

; Visual
DisableProgramGroupPage=yes
DisableWelcomePage=no

; Upgrades: close a running Momentum (window, tray or background hub) first
CloseApplications=force
RestartApplications=no

; Uninstall
UninstallDisplayIcon={app}\{#MyAppExeName}
UninstallDisplayName={#MyAppName}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "quicklaunchicon"; Description: "{cm:CreateQuickLaunchIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "connectides"; Description: "Connect Momentum to the AI IDEs found on this PC (VS Code, Cursor, Windsurf, Antigravity, Claude, Codex...)"; GroupDescription: "IDE integration:"

[Files]
Source: "bridge-ui\build\bin\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion isreadme

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon
Name: "{userappdata}\Microsoft\Internet Explorer\Quick Launch\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: quicklaunchicon

[Run]
; Writes the MCP server entry into each detected IDE's config (other servers are kept).
Filename: "{app}\{#MyAppExeName}"; Parameters: "--ide-connect detected"; Flags: runhidden waituntilterminated; Tasks: connectides
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\{#MyAppExeName}"; Parameters: "--ide-disconnect all"; Flags: runhidden waituntilterminated; RunOnceId: "DisconnectIDEs"

[UninstallDelete]
Type: filesandordirs; Name: "{userappdata}\{#MyAppName}"
