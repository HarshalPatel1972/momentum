; Momentum Installer Script for Inno Setup
; Professional Windows installer with modern UI

#define MyAppName "Momentum"
#define MyAppVersion "1.0.0"
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
; SetupIconFile=bridge-ui\appicon.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern

; Platform
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
PrivilegesRequired=lowest

; Visual
DisableProgramGroupPage=yes
DisableWelcomePage=no

; Uninstall
UninstallDisplayIcon={app}\{#MyAppExeName}
UninstallDisplayName={#MyAppName}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "quicklaunchicon"; Description: "{cm:CreateQuickLaunchIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

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
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: filesandordirs; Name: "{userappdata}\{#MyAppName}"

[Code]
function InitializeSetup(): Boolean;
var
  ResultCode: Integer;
begin
  // Check if app is currently running
  if CheckForMutexes('MomentumApp') then
  begin
    if MsgBox('Momentum is currently running. Please close it before continuing.', mbError, MB_OKCANCEL) = IDOK then
    begin
      Result := False;
    end
    else
      Result := False;
  end
  else
    Result := True;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    // Create .vscode folder example in user's Documents
    // Note: Actual implementation would go here if needed
  end;
end;
