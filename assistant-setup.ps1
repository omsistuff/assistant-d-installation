Echo "Telechargement du programme d'installation"
Import-Module BitsTransfer
Start-BitsTransfer https://static.omsistuff.fr/programs/assistant-setup.exe assistant-setup.tmp.exe
Unblock-File -Path assistant-setup.tmp.exe
Echo "Ouverture du programme. Cliquez sur OUI pour ouvrir en administrateur"
Start-Process -FilePath assistant-setup.tmp.exe -ArgumentList "/S"