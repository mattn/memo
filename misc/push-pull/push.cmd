@echo off
if "x%1" neq "x-usage" goto run
echo push memo to server
goto :eof
:run
setlocal
cd %MEMODIR% && git add -A --ignore-errors && git commit -m update && git push origin master
