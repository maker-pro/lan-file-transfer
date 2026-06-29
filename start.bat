@echo off
setlocal

rem Native Windows launcher. Run it by double-clicking, cmd, or PowerShell.
rem The first argument can be used as the port, for example: start.bat 9090

set "APP_NAME=lan-file-transfer"
set "PORT=0"
set "UPLOAD_DIR=uploads"

if not "%~1"=="" set "PORT=%~1"
if not "%LAN_FILE_PORT%"=="" set "PORT=%LAN_FILE_PORT%"
if not "%LAN_FILE_UPLOAD_DIR%"=="" set "UPLOAD_DIR=%LAN_FILE_UPLOAD_DIR%"

set "SCRIPT_DIR=%~dp0"
cd /d "%SCRIPT_DIR%"

if "%PROCESSOR_ARCHITECTURE%"=="ARM64" (
  set "GOARCH_VALUE=arm64"
) else (
  set "GOARCH_VALUE=amd64"
)

set "BIN_DIR=%SCRIPT_DIR%bin"
set "BIN_PATH=%BIN_DIR%\%APP_NAME%-windows-%GOARCH_VALUE%.exe"

echo OS/ARCH: windows/%GOARCH_VALUE%
if "%PORT%"=="0" (
  echo Port: random available port
) else (
  echo Port: %PORT%
)
echo Upload dir: %UPLOAD_DIR%

if exist "%BIN_PATH%" goto run_app

where go >nul 2>nul
if errorlevel 1 (
  if exist "%SCRIPT_DIR%%APP_NAME%.exe" (
    set "BIN_PATH=%SCRIPT_DIR%%APP_NAME%.exe"
    goto run_app
  )
  echo Go was not found. Install Go, or put a compiled binary here:
  echo %BIN_PATH%
  pause
  exit /b 1
)

echo Binary not found. Building...
if not exist "%BIN_DIR%" mkdir "%BIN_DIR%"
set "GOOS=windows"
set "GOARCH=%GOARCH_VALUE%"
go build -buildvcs=false -o "%BIN_PATH%" .
if errorlevel 1 (
  echo Build failed.
  pause
  exit /b 1
)

:run_app
echo Starting server...
"%BIN_PATH%" -port "%PORT%" -dir "%UPLOAD_DIR%"

endlocal
