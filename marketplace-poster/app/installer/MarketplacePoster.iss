#define MyAppName "Marketplace Poster"
#define MyAppExeName "MarketplacePoster.exe"
#ifndef MyAppVersion
  #define MyAppVersion "2.5.1"
#endif

[Setup]
AppId={{8B7E6C8B-1259-4A2C-8D10-7C5EDE59F2C4}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher=Marketplace Poster
AppPublisherURL=https://marketplace-poster-ai.netlify.app
AppSupportURL=https://marketplace-poster-ai.netlify.app
AppComments=AI-powered Marketplace publishing assistant
DefaultDirName={autopf}\Marketplace Poster
DefaultGroupName=Marketplace Poster
DisableProgramGroupPage=yes
PrivilegesRequired=admin
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=MarketplacePoster-Setup
SetupIconFile=MarketplacePoster.ico
UninstallDisplayIcon={app}\{#MyAppExeName}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
CloseApplications=yes
RestartApplications=no
VersionInfoVersion={#MyAppVersion}
VersionInfoCompany=Marketplace Poster
VersionInfoDescription=Marketplace Poster Setup
VersionInfoProductName=Marketplace Poster
VersionInfoProductVersion={#MyAppVersion}

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: checkedonce
Name: "runasadmin"; Description: "Always run Marketplace Poster as administrator (Windows UAC approval on launch)"; GroupDescription: "Permissions:"; Flags: checkedonce

[Files]
Source: "..\dist\MarketplacePoster.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "MarketplacePoster.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "INSTALL_AI.ps1"; DestDir: "{app}\installer"; Flags: ignoreversion

[Icons]
Name: "{group}\Marketplace Poster"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; IconFilename: "{app}\MarketplacePoster.ico"
Name: "{group}\Install or Repair Smart AI"; Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\installer\INSTALL_AI.ps1"" -InstalledAppDir ""{app}"""; WorkingDir: "{app}"; IconFilename: "{app}\MarketplacePoster.ico"
Name: "{autodesktop}\Marketplace Poster"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; IconFilename: "{app}\MarketplacePoster.ico"; Tasks: desktopicon

[Run]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -Command ""New-Item -Path 'HKCU:\Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers' -Force | Out-Null; New-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers' -Name '{app}\{#MyAppExeName}' -Value '~ RUNASADMIN' -PropertyType String -Force | Out-Null"""; Tasks: runasadmin; Flags: runasoriginaluser runhidden
Filename: "{app}\{#MyAppExeName}"; Description: "Launch Marketplace Poster"; WorkingDir: "{app}"; Flags: nowait postinstall skipifsilent runasoriginaluser

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -Command ""Remove-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers' -Name '{app}\{#MyAppExeName}' -ErrorAction SilentlyContinue"""; Flags: runhidden runasoriginaluser

[UninstallDelete]
Type: filesandordirs; Name: "{app}\installer"

[Code]
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    { Core app is fully installed before any optional AI step is executed. }
  end;
end;
