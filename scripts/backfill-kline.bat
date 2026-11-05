@echo off
setlocal
rem 只重试数据管理里 failed / 僵死 running 的 K 线任务。
rem 不扫描全市场，也不把已完成任务延长到今天。
rem 跑之前先关掉 go-stock，否则桌面进程会用旧逻辑把任务又判失败。
cd /d "%~dp0\.."
set PATH=D:\go\bin;C:\Users\54389\go\bin;%PATH%
set GOCACHE=%CD%\.gocache
echo 补齐失败的 K 线同步任务，库：D:\_installers\go-stock\data\stock.db
go run ./backend/cmd/klinesync -db "D:\_installers\go-stock\data\stock.db" -retry-failed -wait 60m
echo exit=%ERRORLEVEL%
pause
