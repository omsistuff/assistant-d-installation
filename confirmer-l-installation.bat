@echo off
powershell -Command "cd '%~dp0%' ;Start-Process .\assistant-d-installation.exe -Verb runAs"
exit