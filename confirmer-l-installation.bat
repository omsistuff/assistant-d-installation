@echo off
powershell -Command "Start-Process '%~dp0%assistant-d-installation.exe' -Verb runAs"
exit