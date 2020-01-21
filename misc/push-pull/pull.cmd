@echo off
if "x%1" neq "x-usage" goto run
echo pull memo from server
goto :eof
:run
setlocal
cd %MEMODIR% && git pull origin master
