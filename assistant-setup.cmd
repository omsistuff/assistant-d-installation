Powershell.exe -Command "Import-Module BitsTransfer; Start-BitsTransfer https://static.omsistuff.fr/programs/assistant-setup.exe assistant-setup.tmp.exe"
Powershell.exe -Command "Unblock-File -Path assistant-setup.tmp.exe"
Powershell.exe -Command "Start-Process -FilePath assistant-setup.tmp.exe -ArgumentList "/S""
exit
