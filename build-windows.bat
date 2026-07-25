@echo off
setlocal EnableExtensions EnableDelayedExpansion

pushd "%~dp0" >nul
if errorlevel 1 (
  echo ERROR: Unable to enter the project directory.
  exit /b 1
)

set "APP_NAME=DKST Text Flow"
set "BUILD_BIN_DIR=bin"
set "APP_PATH=%CD%\bin\%APP_NAME%.exe"
set "RUN_AFTER_BUILD=0"
set "CREATE_INSTALLER=0"
set "INSTALL_SCOPE=machine"
set "SKIP_TESTS=0"
set "WAILS_BIN="

:parse_args
if "%~1"=="" goto check_tools

if /I "%~1"=="--run" (
  set "RUN_AFTER_BUILD=1"
  shift
  goto parse_args
)

if /I "%~1"=="--installer" (
  set "CREATE_INSTALLER=1"
  set "INSTALL_SCOPE=machine"
  shift
  goto parse_args
)

if /I "%~1"=="--user-installer" (
  set "CREATE_INSTALLER=1"
  set "INSTALL_SCOPE=user"
  shift
  goto parse_args
)

if /I "%~1"=="--skip-tests" (
  set "SKIP_TESTS=1"
  shift
  goto parse_args
)

if /I "%~1"=="-h" goto usage
if /I "%~1"=="--help" goto usage

echo ERROR: Unknown option: %~1
echo.
goto usage_error

:check_tools
where go.exe >nul 2>&1
if errorlevel 1 (
  echo ERROR: Go was not found in PATH.
  goto failed
)

where npm.cmd >nul 2>&1
if errorlevel 1 (
  echo ERROR: npm was not found in PATH.
  goto failed
)

where gcc.exe >nul 2>&1
if errorlevel 1 (
  echo ERROR: GCC was not found in PATH.
  echo        A Windows C compiler is required because SQLite uses CGO.
  goto failed
)

where wails3.exe >nul 2>&1
if not errorlevel 1 set "WAILS_BIN=wails3.exe"

if not defined WAILS_BIN (
  for /f "delims=" %%I in ('go env GOPATH') do set "GO_PATH=%%I"
  if exist "!GO_PATH!\bin\wails3.exe" set "WAILS_BIN=!GO_PATH!\bin\wails3.exe"
)

if not defined WAILS_BIN (
  echo ERROR: wails3 was not found in PATH or GOPATH\bin.
  echo        Install it before running this script.
  goto failed
)

if "%CREATE_INSTALLER%"=="1" (
  where makensis.exe >nul 2>&1
  if errorlevel 1 (
    echo ERROR: makensis was not found in PATH.
    echo        Install NSIS or run without --installer.
    goto failed
  )
)

set "CGO_ENABLED=1"
set "GOOS=windows"

if not exist "%CD%\bin" (
  mkdir "%CD%\bin"
  if errorlevel 1 goto failed
)

if "%SKIP_TESTS%"=="0" (
  echo [1/2] Running Go tests...
  go test ./...
  if errorlevel 1 goto failed
) else (
  echo [1/2] Skipping Go tests.
)

if "%CREATE_INSTALLER%"=="1" (
  echo [2/2] Building the Windows executable and NSIS installer...
  "%WAILS_BIN%" package BIN_DIR=%BUILD_BIN_DIR% CGO_ENABLED=1 INSTALL_SCOPE=%INSTALL_SCOPE%
) else (
  echo [2/2] Building the Windows executable...
  "%WAILS_BIN%" build BIN_DIR=%BUILD_BIN_DIR% CGO_ENABLED=1
)
if errorlevel 1 goto failed

if not exist "%APP_PATH%" (
  echo ERROR: Build output was not found:
  echo        "%APP_PATH%"
  goto failed
)

echo.
echo Build completed:
echo "%APP_PATH%"

if "%RUN_AFTER_BUILD%"=="1" (
  echo Restarting %APP_NAME%...
  powershell.exe -NoProfile -Command "$target = [IO.Path]::GetFullPath('%APP_PATH%'); Get-Process -Name '%APP_NAME%' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $target } | Stop-Process -Force"
  if errorlevel 1 goto failed
  start "" "%APP_PATH%"
  if errorlevel 1 goto failed
)

popd >nul
exit /b 0

:usage
echo Usage: build-windows.bat [--run] [--installer] [--user-installer] [--skip-tests]
echo.
echo Builds the production Windows executable with CGO enabled.
echo.
echo Options:
echo   --run             Restart the executable after a successful build.
echo   --installer       Also create a machine-wide NSIS installer.
echo   --user-installer  Also create a per-user NSIS installer.
echo   --skip-tests      Skip go test ./...
echo   -h, --help        Show this help.
popd >nul
exit /b 0

:usage_error
echo Usage: build-windows.bat [--run] [--installer] [--user-installer] [--skip-tests]
popd >nul
exit /b 2

:failed
echo.
echo Windows build failed.
popd >nul
exit /b 1
