[Setup]
AppId={{5AF5399B-9C51-4716-99D9-DF6B9C23A6ED}
AppName=Assistant installation repaints Omsistuff
AppVersion=1.0
WizardStyle=modern
DefaultDirName={autopf}\Steam\steamapps\common\OMSI 2\
UsePreviousAppDir=no
; Since no icons will be created in "{group}", we don't need the wizard
; to ask for a Start Menu folder name:
DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\assistant-d-installation.exe
Compression=lzma2
SolidCompression=yes
OutputDir=.\release
OutputBaseFilename=assistant-setup
SignedUninstaller=yes
SignTool=MsSign $f

[Files]
Source: "assistant-d-installation.exe"; DestDir: "{app}"; Flags: signonce
Source: "confirmer-l-installation.bat"; DestDir: "{app}"

[Registry]
Root: HKCR; Subkey: "omsistuffinstallassist"; ValueType: string; ValueName: ""; ValueData: "Omsistuff installation automatique"
Root: HKCR; Subkey: "omsistuffinstallassist"; ValueType: string; ValueName: "URL Protocol"; ValueData: ""
Root: HKCR; Subkey: "omsistuffinstallassist\shell"; ValueType: string; ValueName: ""; ValueData: ""
Root: HKCR; Subkey: "omsistuffinstallassist\shell\open"; ValueType: string; ValueName: ""; ValueData: ""
Root: HKCR; Subkey: "omsistuffinstallassist\shell\open\command"; ValueType: string; ValueName: ""; ValueData: "{app}\confirmer-l-installation.bat"

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "francais"; MessagesFile: "compiler:Languages\French.isl"