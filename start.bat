@echo off
rem ============================================================================
rem Centag - Build / Run / Debug Manager (Windows)
rem
rem  start.bat -- Windows aggregate script; command surface aligned with start.sh.
rem
rem -: start.bat [-] [-...]
rem        start.bat [command] --help     view detailed usage of that command
rem        start.bat --version         view version info
rem
rem --:
rem CENTAG_INSTALL_ROOT --(- %USERPROFILE%\.centag)
rem CENTAG_EDITION - personal - minimal, - personal
rem    CENTAG_COLOR=1 or 0   force enable / disable colored output
rem
rem  Encoding: plain ASCII/English (UTF-8 compatible). Opens correctly in both console and IDE.
rem    cmd.exe decodes a batch using the system default codepage at open time; if saved as UTF-8 (with BOM), Chinese turns to mojibake,
rem    and the BOM corrupts the first line so @echo off fails. UTF-8 without BOM is also misread by DBCS and swallows following
rem    characters. (This file used to require GBK; it is now ASCII to avoid all of that.)
rem ============================================================================

setlocal EnableExtensions EnableDelayedExpansion

rem ============================================================================
rem  Command overview (aligned with start.sh):
rem    start.bat debug [personal|minimal] [--desktop]   foreground debug (build + fe hot reload + serve)
rem    start.bat run   [personal|minimal] [--desktop]   start in release mode
rem    start.bat build [be|fe|personal|minimal|team]    build only
rem    start.bat env gen                                 generate config/secrets/.env
rem    start.bat daemon [personal|minimal]              daemon (background resident)
rem  NOTE: this file must stay pure ASCII. cmd.exe decodes a batch with the system default
rem        codepage (GBK on zh-CN systems); non-ASCII bytes (e.g. UTF-8 Chinese) get mispaired
rem        by DBCS decoding and SWALLOW following characters, which corrupts real commands
rem        (observed: "start.bat" turned into "tart.bat" -> "not recognized" error).
rem ============================================================================

rem -- ANSI - ------------------------------------------------------------
set "_ESC="
for /f %%a in ('echo prompt $E^| cmd') do set "_ESC=%%a"

set "USE_COLOR=0"
if defined WT_SESSION set "USE_COLOR=1"
if defined ANSICON set "USE_COLOR=1"
if /i "%CENTAG_COLOR%"=="1" set "USE_COLOR=1"
if /i "%CENTAG_COLOR%"=="0" set "USE_COLOR=0"
if defined NO_COLOR set "USE_COLOR=0"

set "C_RED="
set "C_GREEN="
set "C_YELLOW="
set "C_BLUE="
set "C_CYAN="
set "C_NC="
if "%USE_COLOR%"=="1" (
set "C_RED=%_ESC%[0;31m"
set "C_GREEN=%_ESC%[0;32m"
set "C_YELLOW=%_ESC%[1;33m"
set "C_BLUE=%_ESC%[0;34m"
set "C_CYAN=%_ESC%[0;36m"
set "C_NC=%_ESC%[0m"
)

rem -- -- --------------------------------------------------------------
if not defined GOPROXY set "GOPROXY=https://goproxy.cn,direct"
set "GOTOOLCHAIN=auto"
set "DOCKER_BUILDKIT=1"

rem  project root (strip trailing backslash)
set "PROJECT_ROOT=%~dp0"
if "%PROJECT_ROOT:~-1%"=="\" set "PROJECT_ROOT=%PROJECT_ROOT:~0,-1%"

set "BACKEND_PORT=20060"
set "WEBUI_PORT=5173"
set "EXE_EXT=.exe"
if not defined DAEMON_CHECK_INTERVAL set "DAEMON_CHECK_INTERVAL=5"

set "_FULL_FEATURE_TAGS=protocol_openai,protocol_anthropic,protocol_gemini,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic,backend_gemini,backend_azure"
set "_MINIMAL_TAGS=minimal,protocol_openai,protocol_anthropic,protocol_openairesponses,backend_openai,backend_ollama,backend_anthropic"

rem -- --(- scripts/lib/centag-layout.sh)--------------------------
if not defined CENTAG_EDITION set "CENTAG_EDITION=personal"
if not defined CENTAG_INSTALL_ROOT set "CENTAG_INSTALL_ROOT=%USERPROFILE%\.centag"
call :layout_use_edition "%CENTAG_EDITION%"

rem -- - / - ---------------------------------------------------------
call :resolve_version
set "CENTAG_VERSION=%_RET%"
call :resolve_timestamp
set "VERSION=%_RET_STAMP%"
set "BUILD_TIME=%_RET_NOW%"

rem ============================================================================
rem  top-level dispatch
rem ============================================================================
set "ARG1=%~1"

rem  internal entry: daemon supervise loop
if /i "%ARG1%"=="__daemon" (
call :daemon_supervisor %2 %3 %4 %5 %6
goto :_exit_rc
)

if not defined ARG1 (
call :show_short_help
goto :_exit0
)

if /i "%ARG1%"=="--version" ( call :show_version & goto :_exit0 )
if /i "%ARG1%"=="version" ( call :show_version & goto :_exit0 )
if /i "%ARG1%"=="--help" ( call :show_short_help & goto :_exit0 )
if /i "%ARG1%"=="-h" ( call :show_short_help & goto :_exit0 )
if /i "%ARG1%"=="help" ( call :show_short_help & goto :_exit0 )

set "ARG2=%~2"
if /i "%ARG2%"=="--help" ( call :show_command_help "%ARG1%" & goto :_exit_rc )
if /i "%ARG2%"=="-h" ( call :show_command_help "%ARG1%" & goto :_exit_rc )

if /i "%ARG1%"=="wizard" ( call :cmd_wizard %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="w" ( call :cmd_wizard %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="-w" ( call :cmd_wizard %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="--wizard" ( call :cmd_wizard %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="init" ( call :cmd_init %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="env" ( call :cmd_env %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="build" ( call :cmd_build %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="webui" ( call :cmd_webui %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="run" ( call :cmd_run %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="up" ( call :cmd_run %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="daemon" ( call :cmd_daemon %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="debug" ( call :cmd_debug %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="dev" ( call :cmd_debug %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="stop" ( call :cmd_stop %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="status" ( call :cmd_status & goto :_exit_rc )
if /i "%ARG1%"=="logs" ( call :cmd_logs & goto :_exit_rc )
if /i "%ARG1%"=="clean" ( call :cmd_clean %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="stack" ( call :cmd_stack %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="docker" ( call :cmd_docker %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="pack" ( call :cmd_pack %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="package" ( call :cmd_package %2 %3 %4 %5 %6 %7 %8 %9 & goto :_exit_rc )
if /i "%ARG1%"=="test" ( call :cmd_test & goto :_exit_rc )

call :log_err "--: %ARG1%"
echo.
call :show_short_help
goto :_exit1

rem ============================================================================
rem  common utilities
rem ============================================================================

:log_info
echo %C_BLUE%[INFO]%C_NC% %~1
exit /b 0

:log_ok
echo %C_GREEN%[SUCCESS]%C_NC% %~1
exit /b 0

:log_warn
echo %C_YELLOW%[WARN]%C_NC% %~1
exit /b 0

:log_err
echo %C_RED%[ERROR]%C_NC% %~1
exit /b 0

:log_raw
set "_MSGV=%~1"
echo %C_CYAN%!_MSGV!%C_NC%
exit /b 0

:sleep
set "_SN=%~1"
if not defined _SN set "_SN=1"
timeout /t %_SN% /nobreak >nul 2>&1
if errorlevel 1 (
set /a "_SP=%_SN%+1"
ping -n %_SP% 127.0.0.1 >nul 2>&1
)
exit /b 0

:has_cmd
set "_RET=0"
where %~1 >nul 2>&1
if not errorlevel 1 set "_RET=1"
exit /b 0

rem -- - / - ---------------------------------------------------------
:resolve_version
set "_RET=v0.0.0-dev"
call :has_cmd git
if "%_RET%"=="0" exit /b 0
set "_BR="
for /f "delims=" %%b in ('git rev-parse --abbrev-ref HEAD 2^>nul') do if not defined _BR set "_BR=%%b"
if defined _BR (
set "_P1="
set "_P2="
for /f "tokens=1,2 delims=/" %%x in ("%_BR%") do (
set "_P1=%%x"
set "_P2=%%y"
)
if defined _P2 (
if /i "!_P1!"=="feature" set "_RET=!_P2!"
if /i "!_P1!"=="release" set "_RET=!_P2!"
if /i "!_P1!"=="hotfix" set "_RET=!_P2!"
) else (
if "!_BR:~0,1!"=="v" set "_RET=!_BR!"
)
)
if not "%_RET%"=="v0.0.0-dev" exit /b 0
set "_TAGV2="
for /f "delims=" %%t in ('git describe --tags --abbrev^=0 2^>nul') do set "_TAGV2=%%t"
if defined _TAGV2 set "_RET=%_TAGV2%"
if not defined _RET set "_RET=v0.0.0-dev"
exit /b 0

rem -:wmic(--) -> PowerShell -> %date%/%time%
:resolve_timestamp
set "_RET_STAMP="
set "_D1="
set "_T1="
set "_RET_NOW="
set "_DT="
for /f "skip=1 delims=" %%d in ('wmic os get localdatetime 2^>nul') do if not defined _DT set "_DT=%%d"
if defined _DT goto ts_from_dt
for /f "delims=" %%i in ('powershell -NoProfile -Command Get-Date -f yyyyMMdd-HHmmss 2^>nul') do if not defined _RET_STAMP set "_RET_STAMP=%%i"
for /f "delims=" %%i in ('powershell -NoProfile -Command Get-Date -f yyyy-MM-dd 2^>nul') do if not defined _D1 set "_D1=%%i"
for /f "delims=" %%i in ('powershell -NoProfile -Command Get-Date -f HH:mm:ss 2^>nul') do if not defined _T1 set "_T1=%%i"
if defined _RET_STAMP goto ts_done
set "_HH=%time:~0,2%"
if "%_HH:~0,1%"==" " set "_HH=0%_HH:~1,1%"
set "_RET_STAMP=%date:~0,4%%date:~5,2%%date:~8,2%-%_HH%%time:~3,2%%time:~6,2%"
set "_D1=%date:~0,4%-%date:~5,2%-%date:~8,2%"
set "_T1=%_HH%:%time:~3,2%:%time:~6,2%"
goto ts_done
:ts_from_dt
set "_RET_STAMP=%_DT:~0,8%-%_DT:~8,6%"
set "_D1=%_DT:~0,4%-%_DT:~4,2%-%_DT:~6,2%"
set "_T1=%_DT:~8,2%:%_DT:~10,2%:%_DT:~12,2%"
:ts_done
if defined _D1 if defined _T1 set "_RET_NOW=%_D1% %_T1%"
exit /b 0

rem -- - ------------------------------------------------------------------
:layout_use_edition
set "CENTAG_EDITION=%~1"
if not defined CENTAG_EDITION set "CENTAG_EDITION=personal"
if not defined CENTAG_INSTALL_ROOT set "CENTAG_INSTALL_ROOT=%USERPROFILE%\.centag"
set "CENTAG_BIN_DIR=%CENTAG_INSTALL_ROOT%\bin"
set "CENTAG_LIB_DIR=%CENTAG_INSTALL_ROOT%\lib"
set "CENTAG_VAR_DIR=%CENTAG_INSTALL_ROOT%\var"
set "CENTAG_EDITION_LIB=%CENTAG_LIB_DIR%\%CENTAG_EDITION%"
set "CENTAG_STATIC_DIR=%CENTAG_EDITION_LIB%\static"
set "CENTAG_PACKAGES_DIR=%CENTAG_VAR_DIR%\packages"
set "CENTAG_RELEASE_DIR=%CENTAG_VAR_DIR%\release"
set "CENTAG_CROSS_DIR=%CENTAG_VAR_DIR%\cross"
set "CENTAG_SERVER_BIN=centag-%CENTAG_EDITION%%EXE_EXT%"

rem --
set "BIN_DIR=%CENTAG_EDITION_LIB%"
set "STATIC_DIR=%CENTAG_STATIC_DIR%"
set "PACKAGES_DIR=%CENTAG_PACKAGES_DIR%"
set "SERVER_BIN=%CENTAG_SERVER_BIN%"
exit /b 0

:ensure_dirs
if not exist "%CENTAG_BIN_DIR%" md "%CENTAG_BIN_DIR%"
if not exist "%CENTAG_EDITION_LIB%" md "%CENTAG_EDITION_LIB%"
if not exist "%CENTAG_STATIC_DIR%" md "%CENTAG_STATIC_DIR%"
if not exist "%CENTAG_EDITION_LIB%\storage" md "%CENTAG_EDITION_LIB%\storage"
if not exist "%CENTAG_EDITION_LIB%\logs" md "%CENTAG_EDITION_LIB%\logs"
if not exist "%CENTAG_PACKAGES_DIR%" md "%CENTAG_PACKAGES_DIR%"
if not exist "%CENTAG_RELEASE_DIR%" md "%CENTAG_RELEASE_DIR%"
if not exist "%CENTAG_CROSS_DIR%" md "%CENTAG_CROSS_DIR%"
exit /b 0

rem -- bash -- .sh - --------------------------------------------------
rem  Detect available bash. Prefer Git for Windows (native Windows toolchain), fall back to WSL.
rem  Reason: build-launcher.sh uses BASH_SOURCE to derive ROOT and calls Windows' go for
rem       CGO compile; WSL's bash won't convert D:/xxx args to /d/xxx (errors "No such file"),
rem       and would compile the desktop shell into a Linux target. System32\bash.exe is the WSL launcher.
:find_bash
set "_BASH_TMP="
if exist "%ProgramFiles%\Git\bin\bash.exe" set "_BASH_TMP=%ProgramFiles%\Git\bin\bash.exe"
if not defined _BASH_TMP if exist "%LOCALAPPDATA%\Programs\Git\bin\bash.exe" set "_BASH_TMP=%LOCALAPPDATA%\Programs\Git\bin\bash.exe"
if not defined _BASH_TMP if exist "C://Program Files//Git//bin//bash.exe" set "_BASH_TMP=C://Program Files\Git\bin\bash.exe"
if not defined _BASH_TMP if exist "C://Program Files (x86)//Git//bin//bash.exe" set "_BASH_TMP=C://Program Files (x86)\Git\bin\bash.exe"
rem  Fallback: where bash, but exclude WSL's System32\bash.exe
if not defined _BASH_TMP (
for /f "delims=" %%p in ('where bash 2^>nul ^| findstr /V /I "System32\bash.exe"') do if not defined _BASH_TMP set "_BASH_TMP=%%p"
)
set "_RET=%_BASH_TMP%"
exit /b 0

:need_bash
call :find_bash
if defined _RET exit /b 0
call :log_err "bash not found (Git for Windows). This feature depends on .sh scripts."
call :log_info "  install Git for Windows to run the .sh launcher scripts"
exit /b 1

rem %1 = absolute script path; %2+ are args
:run_sh
call :need_bash
if errorlevel 1 exit /b 1
set "_SH=%~1"
set "_SHARGS="
shift
:run_sh_args
if "%~1"=="" goto run_sh_done
set "_SHARGS=%_SHARGS% %1"
shift
goto run_sh_args
:run_sh_done
"%_RET%" "%_SH%" %_SHARGS%
exit /b %ERRORLEVEL%

rem -- - / - -----------------------------------------------------------
:port_pid
set "_RET="
set "_PP=%~1"
if not defined _PP exit /b 0
for /f "tokens=5" %%p in ('netstat -ano -p TCP ^| findstr LISTENING ^| findstr /C:":%_PP% "') do (
if not defined _RET set "_RET=%%p"
)
exit /b 0

:proc_alive
set "_PA=%~1"
if not defined _PA exit /b 1
set "_PA_OUT="
for /f "delims=" %%l in ('tasklist /FI "PID eq %_PA%" /NH 2^>nul') do set "_PA_OUT=%%l"
if not defined _PA_OUT exit /b 1
echo %_PA_OUT% | find /I "%_PA%" >nul 2>&1
if not errorlevel 1 exit /b 0
exit /b 1

:kill_port
set "_KP=%~1"
if not defined _KP (
    call :log_err "kill_port: missing port number"
exit /b 1
)
call :log_info "checking port %_KP% usage..."
call :port_pid "%_KP%"
if not defined _RET (
call :log_info "  port %_KP% is not in use"
exit /b 0
)
call :log_warn "  killed process on port %_KP% (PID %_RET%)"
taskkill /PID %_RET% /T /F >nul 2>&1
call :sleep 1
call :port_pid "%_KP%"
if defined _RET (
    call :log_err "still cannot free port %_KP% (PID %_RET%)"
exit /b 1
)
call :log_ok "  port %_KP% freed"
exit /b 0

:resolve_backend_port
set /a "_MAXP=%BACKEND_PORT%+10"
for /L %%i in (%BACKEND_PORT%,1,%_MAXP%) do (
call :port_pid "%%i"
if not defined _RET (
if not "%%i"=="%BACKEND_PORT%" (
call :log_warn "  backend port %BACKEND_PORT% is taken, switching to %%i"
set "BACKEND_PORT=%%i"
)
exit /b 0
)
)
call :log_err "  ports %BACKEND_PORT%-%_MAXP% are all occupied"
exit /b 1

:cleanup_residual
set "_CLEANED=0"
call :daemon_stop_quiet
for %%I in (centag.exe centag-personal.exe centag-minimal.exe centag-team.exe) do (
set "_FOUNDIMG="
for /f "delims=" %%l in ('tasklist /FI "IMAGENAME eq %%I" /NH 2^>nul') do set "_FOUNDIMG=%%l"
if defined _FOUNDIMG (
echo !_FOUNDIMG! | find /I "%%I" >nul 2>&1
if not errorlevel 1 (
call :log_warn "  found process: %%I"
taskkill /F /T /IM %%I >nul 2>&1
set "_CLEANED=1"
)
)
)
if exist "%BIN_DIR%\storage\centag.daemon.pid" del /q "%BIN_DIR%\storage\centag.daemon.pid" >nul 2>&1
if exist "%BIN_DIR%\storage\centag.pid" del /q "%BIN_DIR%\storage\centag.pid" >nul 2>&1
if "%_CLEANED%"=="1" (
call :sleep 1
call :log_ok "  cleanup done"
)
exit /b 0

rem -- --- ----------------------------------------------------------
:load_env
set "_STACK_ENV=%PROJECT_ROOT%\deploy\stack\.env"
set "_ENV_FILE=%PROJECT_ROOT%\config\secrets\.env"
set "_MW_FILE=%PROJECT_ROOT%\config\secrets\.env.middleware"

if exist "%_STACK_ENV%" (
call :log_info "  loaded deploy/stack env"
call :load_env_file "%_STACK_ENV%"
if defined POSTGRES_HOST set "PG_HOST=%POSTGRES_HOST%"
if defined POSTGRES_PORT set "PG_PORT=%POSTGRES_PORT%"
if defined POSTGRES_USER set "PG_USER=%POSTGRES_USER%"
if defined POSTGRES_PASSWORD set "PG_PASSWORD=%POSTGRES_PASSWORD%"
if defined POSTGRES_DB set "PG_DATABASE=%POSTGRES_DB%"
call :log_ok "  loaded deploy/stack/.env (PostgreSQL)"
)

if exist "%_ENV_FILE%" (
    call :log_info "loading config/secrets/.env (local config, higher priority)..."
call :load_env_file "%_ENV_FILE%"
call :log_ok "  loaded (config/secrets/.env)"
exit /b 0
)
if exist "%_MW_FILE%" (
    call :log_warn "config/secrets/.env not found, falling back to config/secrets/.env.middleware"
call :load_env_file "%_MW_FILE%"
call :log_ok "  loaded (config/secrets/.env.middleware)"
exit /b 0
)
call :log_warn "config/secrets/.env not found, will start with built-in defaults"
call :log_info "  hint: run 'start.bat env gen' to create config"
exit /b 0

:load_env_file
set "_EF=%~1"
if not exist "%_EF%" exit /b 1
for /f "usebackq delims=" %%L in ("%_EF%") do (
set "_LINE=%%L"
call :load_env_line
)
exit /b 0

:load_env_line
set "_L=%_LINE%"
if not defined _L exit /b 0
for /f "tokens=* delims= " %%s in ("%_L%") do set "_L=%%s"
if not defined _L exit /b 0
if "%_L:~0,1%"=="#" exit /b 0
if /i "%_L:~0,7%"=="export " set "_L=%_L:~7%"
set "_EK="
set "_EV="
for /f "tokens=1,* delims==" %%A in ("%_L%") do (
set "_EK=%%A"
set "_EV=%%B"
)
if not defined _EK exit /b 0
for /f "tokens=* delims= " %%s in ("%_EK%") do set "_EK=%%s"
if not defined _EK exit /b 0
if not defined _EV set "_EV="
set _EV=%_EV:"=%
set "%_EK%=%_EV%"
exit /b 0

:read_env_value
set "_RET="
set "_RF=%~1"
set "_RK=%~2"
if not exist "%_RF%" exit /b 0
for /f "usebackq tokens=1,* delims==" %%A in ("%_RF%") do (
if /i "%%A"=="%_RK%" set "_RET=%%B"
if /i "%%A"=="export %_RK%" set "_RET=%%B"
)
if defined _RET set _RET=%_RET:"=%
exit /b 0

rem -- interactive input --------------------------------------------------------------
rem %1=prompt %2=default y or n; errorlevel 0 means "yes"
:confirm
set "_CP=%~1"
set "_CD=%~2"
if not defined _CD set "_CD=y"
set "_CA="
if /i "%_CD%"=="y" (
set /p "_CA=%C_YELLOW%%_CP% [Y/n]: %C_NC%"
) else (
set /p "_CA=%C_YELLOW%%_CP% [y/N]: %C_NC%"
)
if not defined _CA set "_CA=%_CD%"
if /i "%_CA%"=="y" exit /b 0
if /i "%_CA%"=="yes" exit /b 0
exit /b 1

rem %1=prompt %2=default value; returns _RET
:wizard_read
set "_WP=%~1"
set "_WD=%~2"
set "_WI="
if defined _WD (
set /p "_WI=%C_CYAN%%_WP% [%_WD%]: %C_NC%"
) else (
set /p "_WI=%C_CYAN%%_WP%: %C_NC%"
)
if not defined _WI set "_WI=%_WD%"
set "_RET=%_WI%"
exit /b 0

rem -- -- --------------------------------------------------------------
:check_go
call :has_cmd go
if "%_RET%"=="1" exit /b 0
call :log_err "Go --, -- https://go.dev/dl/"
exit /b 1

:check_docker
call :has_cmd docker
if "%_RET%"=="0" (
call :log_err "Docker not found"
call :log_warn "  get Docker: https://docs.docker.com/get-docker/"
exit /b 1
)
docker info >nul 2>&1
if errorlevel 1 (
call :log_err "Docker not found"
call :log_warn "  use Docker Desktop"
exit /b 1
)
exit /b 0

:compose_cmd
set "_RET="
docker compose version >nul 2>&1
if not errorlevel 1 (
set "_RET=docker compose"
exit /b 0
)
call :has_cmd docker-compose
if "%_RET%"=="1" (
set "_RET=docker-compose"
exit /b 0
)
exit /b 1

:go_arch
set "_GA=amd64"
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "_GA=arm64"
call :has_cmd go
if "%_RET%"=="1" (
set "_GOA="
for /f "delims=" %%a in ('go env GOARCH 2^>nul') do set "_GOA=%%a"
if defined _GOA set "_GA=%_GOA%"
)
set "_RET=%_GA%"
exit /b 0

rem Node.js: WebUI needs 20.19.0+ or 22.12.0+
:node_ok
set "_RET=0"
call :has_cmd node
if "%_RET%"=="0" exit /b 0
set "_NV="
for /f "delims=" %%v in ('node -v 2^>nul') do set "_NV=%%v"
if not defined _NV exit /b 0
set "_NV=%_NV:v=%"
set "_NMAJ=0"
set "_NMIN=0"
set "_NPAT=0"
for /f "tokens=1,2,3 delims=.-" %%a in ("%_NV%") do (
set "_NMAJ=%%a"
set "_NMIN=%%b"
set "_NPAT=%%c"
)
set "_RET=0"
if %_NMAJ% GTR 22 (
set "_RET=1"
exit /b 0
)
if %_NMAJ% EQU 22 if %_NMIN% GEQ 12 set "_RET=1"
if %_NMAJ% EQU 20 if %_NMIN% GEQ 19 set "_RET=1"
exit /b 0

:check_node
call :has_cmd node
if "%_RET%"=="0" (
call :log_err "Node.js not found"
    call :log_warn "WebUI needs Node.js 20.19.0+ or 22.12.0+"
exit /b 1
)
call :has_cmd npm
if "%_RET%"=="0" (
call :log_err "npm not found"
exit /b 1
)
call :node_ok
if "%_RET%"=="1" exit /b 0
set "_CURV="
for /f "delims=" %%v in ('node -v 2^>nul') do set "_CURV=%%v"
call :log_err "Node.js version does not meet WebUI requirement: current %_CURV%, need 20.19.0 or 22.12.0+"
call :log_warn "Windows: recommend nvm-windows: nvm install 22 then nvm use 22"
exit /b 1

:check_dependencies
set "_DEPS_OK=1"
call :log_raw "  checking dependencies..."
echo.
call :has_cmd go
if "%_RET%"=="1" (
set "_GV="
for /f "tokens=3" %%g in ('go version 2^>nul') do set "_GV=%%g"
call :log_ok " Go: !_GV!"
) else (
call :log_err " Go: not found"
call :log_warn "  get Go: https://go.dev/dl/ (Go 1.21+)"
set "_DEPS_OK=0"
)
call :has_cmd docker
if "%_RET%"=="1" (
set "_DV="
for /f "tokens=3" %%d in ('docker --version 2^>nul') do set "_DV=%%d"
if defined _DV set "_DV=%_DV:,=%"
call :log_ok " Docker: !_DV!"
docker info >nul 2>&1
if errorlevel 1 (
call :log_err " Docker: daemon not running"
call :log_warn "  start Docker Desktop"
set "_DEPS_OK=0"
) else (
call :log_ok " Docker: daemon running"
)
) else (
    call :log_warn "  Docker: not installed (optional, for Docker deploy)"
)
call :compose_cmd
if not errorlevel 1 (
call :log_ok " Docker Compose: available"
) else (
    call :log_warn "  Docker Compose: not installed (optional, for middleware management)"
)
call :has_cmd node
if "%_RET%"=="1" (
set "_NV2="
for /f "delims=" %%v in ('node -v 2^>nul') do set "_NV2=%%v"
call :log_ok " Node.js: !_NV2!"
call :has_cmd npm
if "%_RET%"=="1" (
set "_NPMV="
for /f "delims=" %%n in ('npm -v 2^>nul') do set "_NPMV=%%n"
call :log_ok " npm: !_NPMV!"
) else (
call :log_warn " npm: not found"
)
call :node_ok
    if "!_RET!"=="0" call :log_warn "  current Node.js does not meet WebUI requirement (need 20.19.0 or 22.12.0+)"
) else (
    call :log_warn "  Node.js: not installed (optional, for Vue Web UI dev)"
    call :log_warn "     WebUI needs Node.js 20.19.0 or 22.12.0+"
)
echo.
if "%_DEPS_OK%"=="1" (
    call :log_ok "dependency check passed"
exit /b 0
)
call :log_warn "some dependencies missing, some features may be unavailable"
exit /b 1

rem -- -- ------------------------------------------------------------
rem -- -- ----------------------------------------------------------
rem  Auto-detect DB mode: default SQLite; switch to PostgreSQL if PG_HOST / POSTGRES_HOST is set.
:detect_database_mode
if not defined LLM_PROXY_DB_DRIVER set "LLM_PROXY_DB_DRIVER=sqlite"
if /i "%LLM_PROXY_DB_DRIVER%"=="auto" (
if defined PG_HOST (
set "LLM_PROXY_DB_DRIVER=postgresql"
) else (
if defined POSTGRES_HOST (
set "LLM_PROXY_DB_DRIVER=postgresql"
) else (
set "LLM_PROXY_DB_DRIVER=sqlite"
)
)
)
if /i "%LLM_PROXY_DB_DRIVER%"=="postgresql" (
if not defined PG_HOST set "PG_HOST=localhost"
if not defined PG_PORT set "PG_PORT=5432"
if not defined PG_DATABASE set "PG_DATABASE=centag"
call :log_info "--: PostgreSQL"
call :log_info " -: %PG_HOST%:%PG_PORT%/%PG_DATABASE%"
set "_TCP="
for /f "delims=" %%r in ('powershell -NoProfile -Command "$t=New-Object Net.Sockets.TcpClient;try{$t.Connect('%PG_HOST%',%PG_PORT%);$t.Connected}catch{$false};$t.Dispose()" 2^>nul') do set "_TCP=%%r"
if /i not "%_TCP%"=="True" (
echo.
        call :log_err "PostgreSQL unreachable: %PG_HOST%:%PG_PORT%"
echo.
        call :log_info "solution:"
call :log_info " 1. --: start.bat stack start base"
call :log_info " 2. set PG_HOST (in config/secrets/.env)"
call :log_info " 3. -- LLM_PROXY_DB_DRIVER=sqlite - SQLite -"
echo.
exit /b 1
)
exit /b 0
)
if /i "%LLM_PROXY_DB_DRIVER%"=="sqlite" (
rem  SQLite mode: ensure the data dir exists (first run creates the storage dir here)
if not defined SQLITE_PATH set "SQLITE_PATH=%BIN_DIR%\storage\centag.db"
call :log_info "--: SQLite"
call :log_info " -: %SQLITE_PATH%"
for %%F in ("%SQLITE_PATH%") do set "_SDIR=%%~dpF"
if not exist "%_SDIR%" (
md "%_SDIR%" >nul 2>&1
call :log_info "  SQLite: %_SDIR%"
)
exit /b 0
)
call :log_err "  unsupported DB driver: %LLM_PROXY_DB_DRIVER% (expected postgresql or sqlite)"
exit /b 1

rem -- debug console env ----------------------------------------
rem  Override the common LLM_PROXY_LOG_OUTPUT=file from secrets: force debug logs to go
rem  to both console (stdout) and file so access/request logs are visible in the terminal.
rem  Uses set (process env vars); all child processes (incl. desktop launcher) inherit them.
rem  NOTE: centag-desktop.exe on Windows is a GUI subsystem (-H windowsgui); its stdout is
rem  not attached to this console, so the launcher's "tee to console" is lost -- that is
rem  why --desktop additionally mirrors the sidecar log via PowerShell
rem  (see :debug_personal / :debug_minimal).
:debug_console_env
set "LLM_PROXY_SERVER_MODE=debug"
set "LLM_PROXY_LOG_LEVEL=debug"
set "LLM_PROXY_LOG_FORMAT=console"
set "LLM_PROXY_LOG_OUTPUT=both"
if not defined CENTAG_PPROF if not defined LLM_PROXY_PPROF_ENABLED set "CENTAG_PPROF=true"
exit /b 0

:normalize_type
set "_T=%~1"
if /i "%_T%"=="be" ( set "_RET=backend" & exit /b 0 )
if /i "%_T%"=="backend" ( set "_RET=backend" & exit /b 0 )
if /i "%_T%"=="fe" ( set "_RET=frontend" & exit /b 0 )
if /i "%_T%"=="frontend" ( set "_RET=frontend" & exit /b 0 )
if /i "%_T%"=="web" ( set "_RET=frontend" & exit /b 0 )
if /i "%_T%"=="webui" ( set "_RET=frontend" & exit /b 0 )
if /i "%_T%"=="vue" ( set "_RET=frontend" & exit /b 0 )
if /i "%_T%"=="all" ( set "_RET=all" & exit /b 0 )
if /i "%_T%"=="personal" ( set "_RET=personal" & exit /b 0 )
if /i "%_T%"=="minimal" ( set "_RET=minimal" & exit /b 0 )
if /i "%_T%"=="team" ( set "_RET=team" & exit /b 0 )
set "_RET=%_T%"
exit /b 0

:dist_tags
set "_D=%~1"
if /i "%_D%"=="minimal" ( set "_RET=%_MINIMAL_TAGS%" & exit /b 0 )
if /i "%_D%"=="personal" ( set "_RET=%_FULL_FEATURE_TAGS%" & exit /b 0 )
if /i "%_D%"=="team" ( set "_RET=%_FULL_FEATURE_TAGS%" & exit /b 0 )
set "_RET="
exit /b 0

:reject_team
call :log_err "the open-source repo no longer provides the Team build entry (with delegation)."
call :log_info "build in the private repo centag-pro instead (usage aligns with this repo):"
call :log_info " cd ..\centag-pro"
call :log_info " set CENTAG_ROOT=%PROJECT_ROOT%"
call :log_info " start.bat build team"
call :log_info "see centag-pro\README.md"
exit /b 1

rem -- background task wrapper ----------------------------------------------------------
rem usage: set "_BGCMD=original command (may include quotes and redirection)"
rem call :write_bg_bat "- bat -" "-"
:write_bg_bat
(
echo @echo off
echo cd /d "%~2"
echo !_BGCMD!
) > "%~1"
exit /b 0

rem ============================================================================
rem -
rem ============================================================================

rem %1=source dir %2=package path %3=output name centag-[edition] %4=tags %5=ldflags
:compile_go
set "_SRC=%~1"
set "_PKG=%~2"
set "_OUTNAME=%~3"
set "_TAGS=%~4"
set "_LDF=%~5"

set "_ED=%_OUTNAME%"
set "_P1X="
set "_P2X="
for /f "tokens=1,* delims=-" %%a in ("%_OUTNAME%") do (
set "_P1X=%%a"
set "_P2X=%%b"
)
if /i "%_P1X%"=="centag" if defined _P2X set "_ED=%_P2X%"

call :layout_use_edition "%_ED%"
call :ensure_dirs

set "_TAGARG="
if defined _TAGS set "_TAGARG=-tags %_TAGS%"
set "_LDFULL=-s -w"
if defined _LDF set "_LDFULL=-s -w %_LDF%"

call :log_info "- centag-%_ED% ..."
pushd "%_SRC%"
if errorlevel 1 (
    call :log_err "source directory does not exist: %_SRC%"
exit /b 1
)
go mod tidy >nul 2>&1
go build %_TAGARG% -ldflags "%_LDFULL%" -o "%CENTAG_EDITION_LIB%\centag-%_ED%%EXE_EXT%" %_PKG%
set "_RC=%ERRORLEVEL%"
popd
if not "%_RC%"=="0" (
call :log_err "--: centag-%_ED%"
exit /b 1
)
call :write_wrapper "%_ED%"
for %%F in ("%CENTAG_EDITION_LIB%\centag-%_ED%%EXE_EXT%") do set "_SZ=%%~zF"
call :log_ok "centag-%_ED% -- (%_SZ% bytes)"
call :log_info "-: %CENTAG_EDITION_LIB%\centag-%_ED%%EXE_EXT%"
exit /b 0

:write_wrapper
set "_WED=%~1"
if not exist "%CENTAG_BIN_DIR%" md "%CENTAG_BIN_DIR%"
(
echo @echo off
echo set ROOT=%%~dp0..
echo set EDITION=%_WED%
echo set LIB=%%ROOT%%\lib\%%EDITION%%
echo set BIN=%%LIB%%\centag-%%EDITION%%.exe
echo set CENTAG_EDITION=%%EDITION%%
echo if "%%STATIC_PATH%%"=="" set STATIC_PATH=%%LIB%%\static
echo if "%%PROJECT_ROOT%%"=="" set PROJECT_ROOT=%%LIB%%
echo if exist "%%LIB%%\config\profiles\%%EDITION%%\initdata" if "%%INITDATA_PATH%%"=="" set INITDATA_PATH=%%LIB%%\config\profiles\%%EDITION%%\initdata
echo "%%BIN%%" %%*
) > "%CENTAG_BIN_DIR%\centag.cmd"
exit /b 0

rem native implementation of make copy-files
:copy_files
call :ensure_dirs
call :log_info "  copying files..."
if exist "%PROJECT_ROOT%\config\initdata\data\centag.db" (
if not exist "%BIN_DIR%\storage\centag.db" (
if /i "%LLM_PROXY_DB_DRIVER%"=="postgresql" (
call :log_info "Skip SQLite seed (LLM_PROXY_DB_DRIVER is PostgreSQL)"
) else (
copy /y "%PROJECT_ROOT%\config\initdata\data\centag.db" "%BIN_DIR%\storage\centag.db" >nul
call :log_ok "Seeded %BIN_DIR%\storage\centag.db"
)
)
)
if exist "%PROJECT_ROOT%\config\initdata\scripts" (
if not exist "%BIN_DIR%\scripts" md "%BIN_DIR%\scripts"
xcopy /E /I /Y /Q "%PROJECT_ROOT%\config\initdata\scripts\*" "%BIN_DIR%\scripts\" >nul 2>&1
)
if exist "%PROJECT_ROOT%\config\initdata\update" (
if not exist "%BIN_DIR%\update" md "%BIN_DIR%\update"
xcopy /E /I /Y /Q "%PROJECT_ROOT%\config\initdata\update\*" "%BIN_DIR%\update\" >nul 2>&1
)
if exist "%PROJECT_ROOT%\config\initdata\rule" (
if not exist "%BIN_DIR%\rule" md "%BIN_DIR%\rule"
xcopy /E /I /Y /Q "%PROJECT_ROOT%\config\initdata\rule\*" "%BIN_DIR%\rule\" >nul 2>&1
)
if exist "%PROJECT_ROOT%\scripts\*.sh" (
if not exist "%BIN_DIR%\scripts" md "%BIN_DIR%\scripts"
copy /y "%PROJECT_ROOT%\scripts\*.sh" "%BIN_DIR%\scripts\" >nul 2>&1
)
exit /b 0

rem -- -- --------------------------------------------------------------
rem  Build the backend binary (centag-<edition>). go build prints no progress on large
rem  projects and the first build may take minutes; go mod download below splits "module
rem  download" from "compile" so the quiet compile phase does not look like a hang.
:build_backend
call :check_go
if errorlevel 1 exit /b 1
call :layout_use_edition "%CENTAG_EDITION%"
call :ensure_dirs
call :copy_files
call :log_info "  edition=%CENTAG_EDITION%..."
pushd "%PROJECT_ROOT%"
rem  Pre-fetch dependencies (near-instant when cached) so the compile phase is quiet and predictable.
call :log_info "  downloading go modules (if needed)..."
go mod download
set "_LDF=-X 'main.Version=%CENTAG_VERSION%' -X 'main.BuildTime=%BUILD_TIME%'"
call :log_info "  compiling backend (first build may take minutes with no progress output, please wait)..."
go build -tags "%_FULL_FEATURE_TAGS%" -ldflags "-s -w %_LDF%" -o "%BIN_DIR%\%SERVER_BIN%" .\cmd\centag\main.go
set "_RC=%ERRORLEVEL%"
popd
if not "%_RC%"=="0" (
call :log_err "  command failed"
exit /b 1
)
call :write_wrapper "%CENTAG_EDITION%"
call :log_ok "  created: %BIN_DIR%\%SERVER_BIN%"

rem if daemon is running, trigger restart
set "_DPF=%BIN_DIR%\storage\centag.daemon.pid"
set "_SPF=%BIN_DIR%\storage\centag.pid"
set "_DPID="
if exist "%_DPF%" set /p "_DPID=" < "%_DPF%"
if defined _DPID (
call :proc_alive "%_DPID%"
if not errorlevel 1 (
call :log_info "  stopping sidecar process..."
set "_SPID="
if exist "%_SPF%" set /p "_SPID=" < "%_SPF%"
if defined _SPID (
taskkill /PID %_SPID% /T >nul 2>&1
call :log_ok "  sidecar stopped"
)
)
)
exit /b 0

:build_distribution
set "_DIST=%~1"
if not defined _DIST (
call :log_err "  edition required: minimal or personal"
    call :log_info "Team: cd ..\centag-pro then start.bat build team"
exit /b 1
)
if /i "%_DIST%"=="team" (
call :reject_team
exit /b 1
)
if /i "%_DIST%"=="minimal" goto build_dist_ok
if /i "%_DIST%"=="personal" goto build_dist_ok
call :log_err "  unsupported edition: %_DIST% (expected minimal or personal)"
exit /b 1
:build_dist_ok
if not exist "%PROJECT_ROOT%\dist\%_DIST%" (
call :log_err "  build output missing: %PROJECT_ROOT%\dist\%_DIST%"
exit /b 1
)
call :check_go
if errorlevel 1 exit /b 1
call :dist_tags "%_DIST%"
set "_DTAGS=%_RET%"
set "_VLDF=-X 'main.Version=%CENTAG_VERSION%' -X 'main.BuildTime=%BUILD_TIME%'"
call :compile_go "%PROJECT_ROOT%\dist\%_DIST%" "." "centag-%_DIST%" "%_DTAGS%" "%_VLDF%"
exit /b %ERRORLEVEL%

:build_frontend_prod
call :webui_build
exit /b %ERRORLEVEL%

:build_desktop_shell
set "_LS=%PROJECT_ROOT%\scripts\build-launcher.sh"
if not exist "%_LS%" (
    call :log_err "build script not found: %_LS%"
exit /b 1
)
call :need_bash
if errorlevel 1 exit /b 1
call :log_info "  desktop needs Go (no CGO/gcc) on Windows"
rem energye/systray's Windows impl is pure Go (systray_windows.go, uses syscall to call Win32 API),
rem no CGO needed, and no gcc needed; only macOS needs CGO due to systray_darwin.m (Obj-C).
rem build-launcher.sh already uses CGO_ENABLED=0 for windows/linux.
call :run_sh "%_LS%" --desktop
exit /b %ERRORLEVEL%

:resolve_desktop_bin
call :go_arch
set "_GA=%_RET%"
set "_RET="
if exist "%CENTAG_CROSS_DIR%\launcher\windows-%_GA%\centag-desktop.exe" (
set "_RET=%CENTAG_CROSS_DIR%\launcher\windows-%_GA%\centag-desktop.exe"
exit /b 0
)
if exist "%CENTAG_BIN_DIR%\centag-desktop.exe" (
set "_RET=%CENTAG_BIN_DIR%\centag-desktop.exe"
exit /b 0
)
exit /b 1

:build_wrap_shell
set "_WS=%PROJECT_ROOT%\scripts\build-wrap.sh"
if not exist "%_WS%" (
    call :log_err "build script not found: %_WS%"
exit /b 1
)
call :need_bash
if errorlevel 1 exit /b 1
call :log_info "- centag-wrap for windows..."
call :run_sh "%_WS%"
exit /b %ERRORLEVEL%

:resolve_wrap_bin
call :go_arch
set "_GA=%_RET%"
set "_RET="
if exist "%CENTAG_CROSS_DIR%\wrap\windows-%_GA%\centag-wrap.exe" (
set "_RET=%CENTAG_CROSS_DIR%\wrap\windows-%_GA%\centag-wrap.exe"
exit /b 0
)
if exist "%CENTAG_BIN_DIR%\centag-wrap.exe" (
set "_RET=%CENTAG_BIN_DIR%\centag-wrap.exe"
exit /b 0
)
exit /b 1

:build_with_desktop
set "_BWE=%~1"
if /i "%_BWE%"=="team" (
    call :log_err "--desktop does not support team (use Web/Docker for the team edition)"
exit /b 1
)
if /i "%_BWE%"=="personal" goto bwd_ok
if /i "%_BWE%"=="minimal" goto bwd_ok
call :log_err "--desktop -- personal - minimal"
exit /b 1
:bwd_ok
call :log_info "- %_BWE% - + desktop -..."
call :build_distribution "%_BWE%"
if errorlevel 1 exit /b 1
call :build_frontend_prod
if errorlevel 1 exit /b 1
call :build_desktop_shell
if errorlevel 1 exit /b 1
call :log_ok "Ready: centag-%_BWE% + desktop (windows)"
exit /b 0

:build_all_targets
set "_BT=%~1"
if /i "%_BT%"=="all" (
call :log_info "  building all..."
call :webui_build
if errorlevel 1 exit /b 1
call :build_backend
if errorlevel 1 exit /b 1
call :log_ok "  build complete"
exit /b 0
)
if /i "%_BT%"=="backend" (
call :log_info "..."
call :build_backend
exit /b %ERRORLEVEL%
)
if /i "%_BT%"=="webui" (
call :log_info "- Web UI..."
call :webui_build
exit /b %ERRORLEVEL%
)
call :log_err "- build -: %_BT%"
exit /b 1

rem ============================================================================
rem Web UI
rem ============================================================================

:webui_dir_check
if exist "%PROJECT_ROOT%\web" exit /b 0
call :log_err "webui --: %PROJECT_ROOT%\web"
exit /b 1

:webui_dev
call :log_info "starting Web UI dev server..."
call :check_node
if errorlevel 1 exit /b 1
call :webui_dir_check
if errorlevel 1 exit /b 1
pushd "%PROJECT_ROOT%\web"
if not exist "node_modules" (
call :log_info "- Web UI -..."
npm install
if errorlevel 1 (
popd
call :log_err "  build failed"
exit /b 1
)
call :log_ok "  done"
)
call :log_info "  WebUI port: %WEBUI_PORT%..."
call :kill_port "%WEBUI_PORT%"
call :log_info "starting dev server (http://localhost:%WEBUI_PORT%)..."
set "VITE_PORT=%WEBUI_PORT%"
npm run dev
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:webui_build
call :log_info "- Web UI..."
call :check_node
if errorlevel 1 exit /b 1
call :webui_dir_check
if errorlevel 1 exit /b 1
pushd "%PROJECT_ROOT%\web"
set "_NTF=node_modules\.installed-node-version"
set "_CURNODE="
for /f "delims=" %%v in ('node -v 2^>nul') do set "_CURNODE=%%v"
set "_NEED_INSTALL=0"
if not exist "node_modules" set "_NEED_INSTALL=1"
if not exist "%_NTF%" set "_NEED_INSTALL=1"
if "%_NEED_INSTALL%"=="0" (
set "_TAGV="
set /p "_TAGV=" < "%_NTF%"
if not "!_TAGV!"=="%_CURNODE%" set "_NEED_INSTALL=1"
)
if "%_NEED_INSTALL%"=="1" (
call :log_info "  Web UI (Node %_CURNODE%)..."
if exist "node_modules" rd /s /q "node_modules" >nul 2>&1
if exist "package-lock.json" (
npm ci
) else (
npm install
)
if errorlevel 1 (
popd
call :log_err "  build failed"
exit /b 1
)
> "%_NTF%" echo %_CURNODE%
call :log_ok "  done"
)
call :log_info "..."
set "CENTAG_STATIC_DIR=%STATIC_DIR%"
npm run build
set "_RC=%ERRORLEVEL%"
popd
if not "%_RC%"=="0" (
call :log_err "Web UI not built"
exit /b 1
)
call :log_ok "Web UI ready"
call :log_info "  output dir: %STATIC_DIR%"
exit /b 0

:webui_lint
call :log_info "- Web UI -..."
call :check_node
if errorlevel 1 exit /b 1
call :webui_dir_check
if errorlevel 1 exit /b 1
pushd "%PROJECT_ROOT%\web"
if not exist "node_modules" (
call :log_info "- Web UI -..."
npm install
)
npm run lint
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:webui_clean
call :log_info "  Web UI ..."
if exist "%STATIC_DIR%" rd /s /q "%STATIC_DIR%" >nul 2>&1
call :log_ok "  done"
exit /b 0

rem ============================================================================
rem  daemon (native Windows impl)
rem ============================================================================

rem  take own PID (used to write PID file). Leave empty when PowerShell is unavailable,
rem  :daemon_pid falls back to finding it by window title.
:self_pid
set "_SELF_PID="
for /f "delims=" %%p in ('powershell -NoProfile -Command "(Get-CimInstance Win32_Process -Filter ProcessId=$PID).ParentProcessId" 2^>nul') do set "_SELF_PID=%%p"
if not defined _SELF_PID (
for /f "delims=" %%p in ('powershell -NoProfile -Command "(Get-WmiObject Win32_Process -Filter ProcessId=$PID).ParentProcessId" 2^>nul') do set "_SELF_PID=%%p"
)
exit /b 0

rem  internal entry: start.bat __daemon [workdir] [serverbin] [port]
:daemon_supervisor
set "WORK_DIR=%~1"
set "SRV_BIN=%~2"
set "SUP_PORT=%~3"
if not defined WORK_DIR set "WORK_DIR=%BIN_DIR%"
if not defined SRV_BIN set "SRV_BIN=%SERVER_BIN%"
if not defined SUP_PORT set "SUP_PORT=%BACKEND_PORT%"
if not exist "%WORK_DIR%\logs" md "%WORK_DIR%\logs"
if not exist "%WORK_DIR%\storage" md "%WORK_DIR%\storage"
set "DAEMON_LOG=%WORK_DIR%\logs\centag.log"
set "DAEMON_PIDF=%WORK_DIR%\storage\centag.daemon.pid"
set "DAEMON_STOPF=%WORK_DIR%\storage\daemon.stop"
if exist "%DAEMON_STOPF%" del /q "%DAEMON_STOPF%" >nul 2>&1
call :self_pid
if defined _SELF_PID (
> "%DAEMON_PIDF%" echo %_SELF_PID%
) else (
if exist "%DAEMON_PIDF%" del /q "%DAEMON_PIDF%" >nul 2>&1
)
call :log_info "daemon started (PID: %_SELF_PID%, port: %SUP_PORT%, check interval: %DAEMON_CHECK_INTERVAL%s)"
set "_SVCBAT=%WORK_DIR%\storage\centag-service.bat"
set "_BGCMD="%WORK_DIR%\%SRV_BIN%" >> "%DAEMON_LOG%" 2>&1"
call :write_bg_bat "%_SVCBAT%" "%WORK_DIR%"

:daemon_loop
if exist "%DAEMON_STOPF%" goto daemon_exit
call :port_pid "%SUP_PORT%"
if defined _RET (
> "%WORK_DIR%\storage\centag.pid" echo %_RET%
call :sleep %DAEMON_CHECK_INTERVAL%
goto daemon_loop
)
call :log_warn "service not running, restarting..."
start "centag-service" /B cmd /c "%_SVCBAT%"
call :sleep 3
call :port_pid "%SUP_PORT%"
if defined _RET (
call :log_ok "  running (PID: %_RET%)"
) else (
call :log_err "  not running; see %DAEMON_LOG%"
)
goto daemon_loop

:daemon_exit
call :log_info "received stop signal, daemon exiting..."
call :port_pid "%SUP_PORT%"
if defined _RET taskkill /PID %_RET% /T /F >nul 2>&1
if exist "%DAEMON_STOPF%" del /q "%DAEMON_STOPF%" >nul 2>&1
if exist "%DAEMON_PIDF%" del /q "%DAEMON_PIDF%" >nul 2>&1
if exist "%WORK_DIR%\storage\centag.pid" del /q "%WORK_DIR%\storage\centag.pid" >nul 2>&1
if exist "%_SVCBAT%" del /q "%_SVCBAT%" >nul 2>&1
call :log_ok "  done"
exit /b 0

rem  locate daemon PID: prefer PID file, fall back to window-title lookup
rem  output: _RET = PID (empty means not found)
:daemon_pid
set "_RET="
set "_DPFQ=%BIN_DIR%\storage\centag.daemon.pid"
set "_DPQ="
if exist "%_DPFQ%" set /p "_DPQ=" < "%_DPFQ%"
if defined _DPQ (
call :proc_alive "%_DPQ%"
if not errorlevel 1 (
set "_RET=%_DPQ%"
exit /b 0
)
)
for /f "tokens=2 delims=," %%p in ('tasklist /V /FO CSV /NH 2^>nul ^| findstr /C:"Centag Daemon"') do (
if not defined _RET set "_RET=%%~p"
)
exit /b 0

:daemon_stop_quiet
call :daemon_pid
set "_DPID2=%_RET%"
if not defined _DPID2 (
if exist "%BIN_DIR%\storage\centag.daemon.pid" del /q "%BIN_DIR%\storage\centag.daemon.pid" >nul 2>&1
exit /b 0
)
> "%BIN_DIR%\storage\daemon.stop" echo stop
taskkill /PID %_DPID2% /T /F >nul 2>&1
if exist "%BIN_DIR%\storage\centag.daemon.pid" del /q "%BIN_DIR%\storage\centag.daemon.pid" >nul 2>&1
exit /b 0

:daemon_stop
call :daemon_pid
set "_DPID3=%_RET%"
if not defined _DPID3 (
if exist "%BIN_DIR%\storage\centag.daemon.pid" del /q "%BIN_DIR%\storage\centag.daemon.pid" >nul 2>&1
if exist "%BIN_DIR%\storage\daemon.stop" del /q "%BIN_DIR%\storage\daemon.stop" >nul 2>&1
call :log_info "  removing daemon state"
exit /b 0
)
call :log_info "  PID: %_DPID3%..."
> "%BIN_DIR%\storage\daemon.stop" echo stop
taskkill /PID %_DPID3% /T /F >nul 2>&1
call :sleep 1
if exist "%BIN_DIR%\storage\centag.daemon.pid" del /q "%BIN_DIR%\storage\centag.daemon.pid" >nul 2>&1
if exist "%BIN_DIR%\storage\daemon.stop" del /q "%BIN_DIR%\storage\daemon.stop" >nul 2>&1
call :log_ok "  done"
exit /b 0

:daemon_status
call :daemon_pid
if defined _RET (
call :log_ok "  running (PID: %_RET%)"
exit /b 0
)
call :log_warn "  not running"
exit /b 0

:daemon_start
call :load_env
call :detect_database_mode
if errorlevel 1 exit /b 1
call :resolve_backend_port
if errorlevel 1 exit /b 1
if not exist "%BIN_DIR%\%SERVER_BIN%" (
    call :log_info "%SERVER_BIN% not found, building first..."
call :build_backend
if errorlevel 1 exit /b 1
)
call :ensure_dirs
call :daemon_stop
call :log_info "  dir: %BIN_DIR%..."
start "Centag Daemon" /MIN cmd /c ""%~f0" __daemon "%BIN_DIR%" "%SERVER_BIN%" "%BACKEND_PORT%""
set "_WAITED=0"
:daemon_wait_pid
if %_WAITED% GEQ 20 goto daemon_wait_done
call :sleep 1
set /a "_WAITED+=1"
call :daemon_pid
if defined _RET goto daemon_wait_done
goto daemon_wait_pid
:daemon_wait_done
call :daemon_pid
if defined _RET (
call :log_ok "  running (PID: %_RET%)"
) else (
call :log_warn "  not running; log: %BIN_DIR%\logs\centag.log"
)
call :print_test_examples
call :log_info "--: start.bat daemon stop"
call :log_info "--: start.bat daemon status"
exit /b 0

:daemon_debug
call :load_env
call :detect_database_mode
if errorlevel 1 exit /b 1
call :debug_console_env
call :resolve_backend_port
if errorlevel 1 exit /b 1
if not exist "%BIN_DIR%\%SERVER_BIN%" (
call :build_backend
if errorlevel 1 exit /b 1
)
call :print_test_examples
call :log_info "starting in debug mode (foreground)..."
pushd "%BIN_DIR%"
"%BIN_DIR%\%SERVER_BIN%"
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

rem ============================================================================
rem - / - / -
rem ============================================================================

:stop_backend_only
call :log_info "  stopping backend..."
call :daemon_stop_quiet
call :kill_port "%BACKEND_PORT%"
for %%I in (centag.exe centag-personal.exe centag-minimal.exe) do taskkill /F /T /IM %%I >nul 2>&1
if exist "%BIN_DIR%\storage\centag.pid" del /q "%BIN_DIR%\storage\centag.pid" >nul 2>&1
call :log_ok "  stopped"
exit /b 0

:stop_all_services
call :log_info "stopping all services..."
call :daemon_stop
call :kill_port "%BACKEND_PORT%"
for %%I in (centag.exe centag-personal.exe centag-minimal.exe centag-desktop.exe) do taskkill /F /T /IM %%I >nul 2>&1
call :sleep 1
call :port_pid "%BACKEND_PORT%"
if defined _RET (
call :log_err "- %BACKEND_PORT% -- PID %_RET% -"
exit /b 1
)
call :log_ok "all services stopped"
exit /b 0

rem ============================================================================
rem  CLI implementation
rem ============================================================================

:cmd_init
call :log_info "initializing dev environment..."
call :check_go
if errorlevel 1 exit /b 1
call :log_info "- Go -..."
call :log_ok "Go proxy: %GOPROXY%"
call :log_info "..."
pushd "%PROJECT_ROOT%"
go mod tidy
set "_RC=%ERRORLEVEL%"
popd
if not "%_RC%"=="0" (
call :log_err "  build failed"
exit /b 1
)
call :ensure_dirs
call :copy_files
call :log_ok "  done"
exit /b 0

:cmd_env
set "_ESUB=%~1"
if not defined _ESUB set "_ESUB=gen"
if /i not "%_ESUB%"=="gen" (
call :log_err "- env -: %_ESUB%"
call :log_info "-: start.bat env gen [--interactive] [--force]"
exit /b 1
)
set "_GSH=%PROJECT_ROOT%\scripts\generate-secrets.sh"
if not exist "%_GSH%" set "_GSH=%PROJECT_ROOT%\scripts\ops\generate-secrets.sh"
if not exist "%_GSH%" (
call :log_err "generate-secrets script not found"
exit /b 1
)
call :run_sh "%_GSH%" %2 %3 %4 %5 %6 %7 %8 %9
exit /b %ERRORLEVEL%

:cmd_build
set "BTARGET=all"
set "BTARGET_SET=0"
set "WITH_DESKTOP=0"
set "WITH_DOCKER=0"
set "WITH_WRAP=0"
:cmd_build_parse
if "%~1"=="" goto cmd_build_done
if /i "%~1"=="--desktop" (
set "WITH_DESKTOP=1"
shift
goto cmd_build_parse
)
if /i "%~1"=="--docker" (
set "WITH_DOCKER=1"
shift
goto cmd_build_parse
)
if /i "%~1"=="--wrap" (
set "WITH_WRAP=1"
shift
goto cmd_build_parse
)
if "%BTARGET_SET%"=="0" (
set "BTARGET=%~1"
set "BTARGET_SET=1"
shift
goto cmd_build_parse
)
call :log_err "- build -: %~1"
call :log_info "-: start.bat build [-] [--desktop] [--docker] [--wrap]"
exit /b 1
:cmd_build_done
call :normalize_type "%BTARGET%"
set "BTARGET=%_RET%"

if "%WITH_DESKTOP%"=="1" (
if "%WITH_DOCKER%"=="1" (
        call :log_err "--desktop and --docker cannot be used together"
exit /b 1
)
)

rem  dev build
if /i "%BTARGET%"=="backend" goto cmd_build_dev
if /i "%BTARGET%"=="frontend" goto cmd_build_dev
if /i "%BTARGET%"=="all" goto cmd_build_dev
if /i "%BTARGET%"=="wrap" goto cmd_build_wrap
if /i "%BTARGET%"=="personal" goto cmd_build_dist
if /i "%BTARGET%"=="minimal" goto cmd_build_dist
if /i "%BTARGET%"=="team" goto cmd_build_dist
call :log_err "  unsupported build target: %BTARGET%"
call :log_info "supported targets: all be fe personal minimal wrap (Team goes to centag-pro)"
call :log_info "-: --desktop --docker --wrap"
exit /b 1

:cmd_build_dev
if "%WITH_DESKTOP%"=="1" (
    call :log_err "--desktop cannot be used for build %BTARGET%; use build personal --desktop"
exit /b 1
)
if "%WITH_DOCKER%"=="1" (
    call :log_err "--docker cannot be used for build %BTARGET%; use build personal --docker"
exit /b 1
)
if "%WITH_WRAP%"=="1" (
    call :log_err "--wrap cannot be used for build %BTARGET%; use build wrap"
exit /b 1
)
if /i "%BTARGET%"=="backend" (
call :build_all_targets backend
exit /b !ERRORLEVEL!
)
if /i "%BTARGET%"=="frontend" (
call :build_all_targets webui
exit /b !ERRORLEVEL!
)
call :build_all_targets all
exit /b !ERRORLEVEL!

:cmd_build_wrap
if "%WITH_DESKTOP%"=="1" (
    call :log_err "--desktop cannot be used together with build wrap"
exit /b 1
)
if "%WITH_DOCKER%"=="1" (
    call :log_err "--docker cannot be used together with build wrap"
exit /b 1
)
call :build_wrap_shell
if errorlevel 1 exit /b 1
call :log_ok "Ready: centag-wrap (windows)"
call :log_info "real source command: cd apps\\wrap then GOWORK=off go build -o centag-wrap.exe ."
exit /b 0

:cmd_build_dist
set "_BDE=%BTARGET%"
if "%WITH_DOCKER%"=="1" (
call :dist_docker_build "%_BDE%"
exit /b !ERRORLEVEL!
)
if "%WITH_DESKTOP%"=="1" (
call :build_with_desktop "%_BDE%"
exit /b !ERRORLEVEL!
)
call :build_distribution "%_BDE%"
if errorlevel 1 exit /b 1
if "%WITH_WRAP%"=="1" call :build_wrap_shell
exit /b 0

:cmd_webui
set "_WSUB=%~1"
if not defined _WSUB set "_WSUB=build"
if /i "%_WSUB%"=="dev" (
call :webui_dev
exit /b !ERRORLEVEL!
)
if /i "%_WSUB%"=="build" (
call :webui_build
exit /b !ERRORLEVEL!
)
if /i "%_WSUB%"=="lint" (
call :webui_lint
exit /b !ERRORLEVEL!
)
if /i "%_WSUB%"=="clean" (
call :webui_clean
exit /b !ERRORLEVEL!
)
call :log_err "- webui -: %_WSUB%"
call :log_info "-: start.bat webui dev - build - lint - clean"
exit /b 1

:cmd_run
set "RSVC=backend"
set "RSVC_SET=0"
set "R_DESKTOP=0"
set "R_DOCKER=0"
set "R_BG=0"
set "R_EXTRA="
:cmd_run_parse
if "%~1"=="" goto cmd_run_done
if /i "%~1"=="--desktop" (
set "R_DESKTOP=1"
shift
goto cmd_run_parse
)
if /i "%~1"=="--docker" (
set "R_DOCKER=1"
shift
goto cmd_run_parse
)
if /i "%~1"=="--background" (
set "R_BG=1"
shift
goto cmd_run_parse
)
if /i "%~1"=="-b" (
set "R_BG=1"
shift
goto cmd_run_parse
)
if "%RSVC_SET%"=="0" (
set "RSVC=%~1"
set "RSVC_SET=1"
shift
goto cmd_run_parse
)
set "R_EXTRA=%R_EXTRA% %1"
shift
goto cmd_run_parse
:cmd_run_done
call :normalize_type "%RSVC%"
set "RSVC=%_RET%"

if /i "%RSVC%"=="backend" goto cmd_run_dev_svc
if /i "%RSVC%"=="frontend" goto cmd_run_dev_svc
if /i "%RSVC%"=="all" goto cmd_run_dev_svc
if /i "%RSVC%"=="wrap" goto cmd_run_wrap
if /i "%RSVC%"=="team" goto cmd_run_team
if /i "%RSVC%"=="personal" goto cmd_run_edition
if /i "%RSVC%"=="minimal" goto cmd_run_edition
call :log_err "  unsupported service: %RSVC%"
call :log_info "-: be fe all personal minimal wrap"
call :log_info "-: --desktop --docker --background"
exit /b 1

:cmd_run_dev_svc
if "%R_DESKTOP%"=="1" (
    call :log_err "--desktop does not apply to run %RSVC%"
exit /b 1
)
if "%R_DOCKER%"=="1" (
    call :log_err "--docker does not apply to run %RSVC%"
exit /b 1
)
if /i "%RSVC%"=="frontend" (
call :webui_dev
exit /b !ERRORLEVEL!
)
if /i "%RSVC%"=="all" (
call :run_all_dev
exit /b !ERRORLEVEL!
)
if "%R_BG%"=="1" (
call :daemon_start
exit /b !ERRORLEVEL!
)
call :run_backend_fg
exit /b !ERRORLEVEL!

:cmd_run_wrap
call :resolve_wrap_bin
if errorlevel 1 (
    call :log_info "centag-wrap not found, building first..."
call :build_wrap_shell
if errorlevel 1 exit /b 1
call :resolve_wrap_bin
if errorlevel 1 (
        call :log_err "centag-wrap still not found after build"
exit /b 1
)
)
call :log_info "-: %_RET%%R_EXTRA%"
%_RET%%R_EXTRA%
exit /b %ERRORLEVEL%

:cmd_run_team
call :log_err "team is only supported in the private repo centag-pro; use personal or minimal in this repo"
call :log_info "example: start.bat run personal   or   cd ..\\centag-pro then start.bat run team"
exit /b 1

:cmd_run_edition
if "%R_DOCKER%"=="1" (
if "%R_DESKTOP%"=="1" (
        call :log_err "--desktop and --docker cannot be used together"
exit /b 1
)
call :dist_docker_run "%RSVC%" "" "" "false"
exit /b !ERRORLEVEL!
)
if "%R_DESKTOP%"=="1" (
call :run_edition_desktop "%RSVC%"
exit /b !ERRORLEVEL!
)
if "%R_BG%"=="1" (
call :log_info " %RSVC% CLI (subcommand)"
call :layout_use_edition "%RSVC%"
call :daemon_start
exit /b !ERRORLEVEL!
)
call :run_edition_fg "%RSVC%"
exit /b !ERRORLEVEL!

:run_edition_fg
set "_RE=%~1"
call :layout_use_edition "%_RE%"
set "_SIDECAR=%BIN_DIR%\centag-%_RE%%EXE_EXT%"
if not exist "%_SIDECAR%" (
    call :log_info "centag-%_RE% not found, building first..."
call :build_distribution "%_RE%"
if errorlevel 1 exit /b 1
)
call :load_env
set "INITDATA_PATH=%PROJECT_ROOT%\config\initdata"
set "CENTAG_PRICING_FILE=%PROJECT_ROOT%\config\pricing\default.yaml"
if not defined LLM_PROXY_ADMIN_PASSWORD (
    call :log_warn "LLM_PROXY_ADMIN_PASSWORD not detected; first startup will set the admin password via the init wizard"
)
call :log_info "starting %_RE% CLI (foreground): %_SIDECAR%"
call :log_info "--: start.bat run %_RE% --background"
call :log_info "- Ctrl+C -"
set "CENTAG_EDITION=%_RE%"
pushd "%BIN_DIR%"
"%_SIDECAR%"
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:run_edition_desktop
set "_RDE=%~1"
call :layout_use_edition "%_RDE%"
set "_SIDECAR2=%BIN_DIR%\centag-%_RDE%%EXE_EXT%"
if not exist "%_SIDECAR2%" (
    call :log_info "centag-%_RDE% not found, building first..."
call :build_distribution "%_RDE%"
if errorlevel 1 exit /b 1
)
call :load_env
set "INITDATA_PATH=%PROJECT_ROOT%\config\initdata"
set "CENTAG_PRICING_FILE=%PROJECT_ROOT%\config\pricing\default.yaml"
if not exist "%BIN_DIR%\static\index.html" (
    call :log_info "building frontend static assets ..."
call :build_frontend_prod
if errorlevel 1 exit /b 1
)
call :resolve_desktop_bin
if errorlevel 1 (
    call :log_info "desktop shell not found, building first..."
call :build_desktop_shell
if errorlevel 1 exit /b 1
call :resolve_desktop_bin
if errorlevel 1 (
        call :log_err "desktop shell still not found after build"
exit /b 1
)
)
set "_DTOPBIN=%_RET%"
if not defined LLM_PROXY_ADMIN_PASSWORD (
    call :log_warn "LLM_PROXY_ADMIN_PASSWORD not detected; first startup will set the admin password via the init wizard"
) else (
    call :log_info "admin password env var loaded (from config\\secrets\\.env)"
)
call :log_info "- desktop edition=%_RDE% platform=windows"
call :log_info " desktop: %_DTOPBIN%"
call :log_info " sidecar: %_SIDECAR2%"
"%_DTOPBIN%" -edition="%_RDE%" -bin=%_SIDECAR2%%R_EXTRA%
exit /b %ERRORLEVEL%

:run_backend_fg
call :load_env
call :resolve_backend_port
if errorlevel 1 exit /b 1
call :layout_use_edition "%CENTAG_EDITION%"
if not exist "%BIN_DIR%\%SERVER_BIN%" (
    call :log_info "%SERVER_BIN% not found, building first..."
call :build_backend
if errorlevel 1 exit /b 1
)
call :print_test_examples
call :log_info "  starting backend (%BIN_DIR%, port %BACKEND_PORT%)..."
pushd "%BIN_DIR%"
"%SERVER_BIN%" serve
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:run_all_dev
call :load_env
call :detect_database_mode
if errorlevel 1 exit /b 1
call :debug_console_env
call :resolve_backend_port
if errorlevel 1 exit /b 1
if not exist "%BIN_DIR%\%SERVER_BIN%" (
call :build_backend
if errorlevel 1 exit /b 1
)
if not exist "%BIN_DIR%\logs" md "%BIN_DIR%\logs"
call :log_info "  starting in background (port %BACKEND_PORT%)..."
set "_BEBAT=%BIN_DIR%\storage\centag-backend.bat"
set "_BGCMD="%BIN_DIR%\%SERVER_BIN%" serve >> "%BIN_DIR%\logs\centag.log" 2>&1"
call :write_bg_bat "%_BEBAT%" "%BIN_DIR%"
start "centag-backend" /B cmd /c "%_BEBAT%"
call :sleep 3
call :port_pid "%BACKEND_PORT%"
if not defined _RET (
call :log_err "  failed to start; log: %BIN_DIR%\logs\centag.log"
exit /b 1
)
call :log_ok "  started (PID: %_RET%)"
call :log_info "starting frontend dev server..."
call :webui_dev
exit /b !ERRORLEVEL!

:cmd_daemon
set "_DSUB=%~1"
if not defined _DSUB set "_DSUB=backend"
if /i "%_DSUB%"=="stop" (
call :daemon_stop
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="status" (
call :daemon_status
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="debug" (
call :daemon_debug
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="backend" (
call :daemon_start
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="be" (
call :daemon_start
exit /b !ERRORLEVEL!
)
call :log_err "- daemon -: %_DSUB%"
call :log_info "-: start.bat daemon backend - stop - debug - status"
exit /b 1

rem -- debug -- ------------------------------------------------------
rem  debug <edition> [--desktop] [--docker]
rem    --desktop  tray+sidecar form (product desktop); --docker starts via containers.
:cmd_debug
set "D_EDITION=personal"
set "D_DESKTOP=0"
set "D_DOCKER=0"
:cmd_debug_parse
if "%~1"=="" goto cmd_debug_done
if /i "%~1"=="--desktop" (
set "D_DESKTOP=1"
shift
goto cmd_debug_parse
)
if /i "%~1"=="--docker" (
set "D_DOCKER=1"
shift
goto cmd_debug_parse
)
if /i "%~1"=="personal" (
set "D_EDITION=personal"
shift
goto cmd_debug_parse
)
if /i "%~1"=="minimal" (
set "D_EDITION=minimal"
shift
goto cmd_debug_parse
)
if /i "%~1"=="team" (
set "D_EDITION=team"
shift
goto cmd_debug_parse
)
shift
goto cmd_debug_parse
:cmd_debug_done
if /i "%D_EDITION%"=="team" (
if "%D_DESKTOP%"=="1" (
call :log_err "--desktop - team"
exit /b 1
)
call :reject_team
exit /b 1
)
if "%D_DOCKER%"=="1" (
call :debug_docker "%D_EDITION%"
exit /b !ERRORLEVEL!
)
if /i "%D_EDITION%"=="minimal" (
call :debug_minimal "%D_DESKTOP%"
exit /b !ERRORLEVEL!
)
call :debug_personal "%D_DESKTOP%"
exit /b !ERRORLEVEL!

:debug_personal
set "_DP_DESK=%~1"
call :load_env
call :detect_database_mode
if errorlevel 1 exit /b 1
call :debug_console_env
call :layout_use_edition personal
set "INITDATA_PATH=%PROJECT_ROOT%\config\initdata"
set "CENTAG_PRICING_FILE=%PROJECT_ROOT%\config\pricing\default.yaml"
call :cleanup_residual
if exist "%BIN_DIR%\storage\centag.pid" del /q "%BIN_DIR%\storage\centag.pid" >nul 2>&1
call :resolve_backend_port
if errorlevel 1 exit /b 1
call :check_go
if errorlevel 1 exit /b 1
call :log_info "  edition=personal..."
call :build_backend
if errorlevel 1 exit /b 1
if "%_DP_DESK%"=="1" (
call :log_info "- desktop -..."
call :build_desktop_shell
if errorlevel 1 exit /b 1
)
call :check_node
if errorlevel 1 exit /b 1
if not exist "%PROJECT_ROOT%\web\node_modules" (
call :log_info "- Web UI -..."
pushd "%PROJECT_ROOT%\web"
npm install
set "_RC=%ERRORLEVEL%"
popd
if not "!_RC!"=="0" (
call :log_err "  build failed"
exit /b 1
)
)
if not exist "%BIN_DIR%\static" md "%BIN_DIR%\static"

call :log_info "starting frontend watch build (refresh browser after changes to take effect)..."
set "_VITELOG=%TEMP%\centag-vite.log"
set "_VITEBAT=%TEMP%\centag-vite-watch.bat"
set "_BGCMD=call npx.cmd vite build --watch --outDir "%BIN_DIR%\static" --emptyOutDir false >> "%_VITELOG%" 2>&1"
call :write_bg_bat "%_VITEBAT%" "%PROJECT_ROOT%\web"
start "centag-vite-watch" /MIN cmd /c "%_VITEBAT%"
call :sleep 6
call :log_ok "  frontend watch started (log: %_VITELOG%)"

echo.
call :log_info "========================================"
call :log_info "  dev mode started"
call :log_info "  product edition: personal"
if "%_DP_DESK%"=="1" (
    call :log_info "  form:        sidecar (all logs in this console)"
) else (
    call :log_info "  form:        cli (foreground sidecar)"
)
call :log_info " --: http://localhost:%BACKEND_PORT%"
if "%CENTAG_PPROF%"=="true" call :log_info " pprof: http://127.0.0.1:6060/debug/pprof/"
call :log_info "  after frontend changes: refresh browser to see latest content"
call :log_info "  after backend changes: re-run start.bat debug will auto-compile first"
call :log_info "  press Ctrl+C to stop"
call :log_info "========================================"
echo.

if "%_DP_DESK%"=="1" (
call :resolve_desktop_bin
if not errorlevel 1 if defined _RET (
    call :log_info "  launching desktop tray (--no-sidecar)..."
    start "centag-desktop" "!_RET!" -edition=personal -bin=%BIN_DIR%\centag-personal%EXE_EXT% -no-sidecar -no-open
    call :sleep 1
)
call :log_info "  sidecar running in this console (all logs visible below, Ctrl+C to stop)"
echo.
set "CENTAG_EDITION=personal"
pushd "%BIN_DIR%"
"%SERVER_BIN%" serve
set "_RC=%ERRORLEVEL%"
popd
if defined _DESK_PID taskkill /PID %_DESK_PID% /T /F >nul 2>&1
exit /b %_RC%
)
set "CENTAG_EDITION=personal"
pushd "%BIN_DIR%"
"%SERVER_BIN%" serve
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:debug_minimal
set "_DM_DESK=%~1"
call :load_env
call :detect_database_mode
if errorlevel 1 exit /b 1
call :debug_console_env
call :layout_use_edition minimal
set "INITDATA_PATH=%PROJECT_ROOT%\config\initdata"
set "CENTAG_PRICING_FILE=%PROJECT_ROOT%\config\pricing\default.yaml"
call :cleanup_residual
if exist "%BIN_DIR%\storage\centag.pid" del /q "%BIN_DIR%\storage\centag.pid" >nul 2>&1
call :resolve_backend_port
if errorlevel 1 exit /b 1
call :check_go
if errorlevel 1 exit /b 1
call :log_info "  building minimal distribution..."
call :build_distribution minimal
if errorlevel 1 exit /b 1
if "%_DM_DESK%"=="1" (
call :log_info "- desktop -..."
call :build_desktop_shell
if errorlevel 1 exit /b 1
)
call :check_node
if errorlevel 1 exit /b 1
if not exist "%PROJECT_ROOT%\web\node_modules" (
call :log_info "- Web UI -..."
pushd "%PROJECT_ROOT%\web"
npm install
set "_RC=%ERRORLEVEL%"
popd
if not "!_RC!"=="0" (
call :log_err "  build failed"
exit /b 1
)
)
if not exist "%BIN_DIR%\static" md "%BIN_DIR%\static"
call :log_info "building minimal admin console frontend ..."
pushd "%PROJECT_ROOT%\web"
npx.cmd vite build --outDir "%BIN_DIR%\static" --emptyOutDir true
set "_RC=%ERRORLEVEL%"
popd
if not "!_RC!"=="0" (
call :log_err "  build failed"
exit /b 1
)
if not exist "%BIN_DIR%\static\index.html" (
call :log_err "  frontend not built (missing %BIN_DIR%\static\index.html)"
exit /b 1
)
call :log_ok "  done"

call :log_info "starting frontend watch build (refresh browser after changes)..."
set "_VITELOG2=%TEMP%\centag-vite-minimal.log"
set "_VITEBAT2=%TEMP%\centag-vite-minimal.bat"
set "_BGCMD=call npx.cmd vite build --watch --outDir "%BIN_DIR%\static" --emptyOutDir false >> "%_VITELOG2%" 2>&1"
call :write_bg_bat "%_VITEBAT2%" "%PROJECT_ROOT%\web"
start "centag-vite-minimal" /MIN cmd /c "%_VITEBAT2%"

echo.
call :log_info "========================================"
call :log_info "  running in minimal mode"
if "%_DM_DESK%"=="1" (
    call :log_info "  form:        sidecar (all logs in this console)"
) else (
    call :log_info "  form:        cli (foreground sidecar)"
)
call :log_info " -: WebUI (edition=minimal)"
call :log_info " --: http://localhost:%BACKEND_PORT%/static/"
if "%CENTAG_PPROF%"=="true" call :log_info " pprof: http://127.0.0.1:6060/debug/pprof/"
call :log_info "  first entry: set admin password then log in"
call :log_info " - Ctrl+C -"
call :log_info "========================================"
echo.

if "%_DM_DESK%"=="1" (
call :resolve_desktop_bin
if not errorlevel 1 if defined _RET (
    call :log_info "  launching desktop tray (--no-sidecar)..."
    start "centag-desktop" "!_RET!" -edition=minimal -bin=%BIN_DIR%\centag-minimal%EXE_EXT% -no-sidecar -no-open
    call :sleep 1
)
call :log_info "  sidecar running in this console (all logs visible below, Ctrl+C to stop)"
echo.
set "CENTAG_EDITION=minimal"
pushd "%BIN_DIR%"
"%SERVER_BIN%"
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%
)
set "CENTAG_EDITION=minimal"
pushd "%BIN_DIR%"
"%SERVER_BIN%"
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:debug_docker
set "_DD=%~1"
call :load_env
call :log_info "Docker --: %_DD%"
echo.
call :check_docker
if errorlevel 1 exit /b 1
call :dist_docker_build "%_DD%"
if errorlevel 1 exit /b 1
docker ps -a --format "{{.Names}}" 2>nul | findstr /B /E /C:"centag-%_DD%" >nul 2>&1
if not errorlevel 1 (
call :log_info "  removing container centag-%_DD%..."
docker rm -f "centag-%_DD%" >nul 2>&1
call :sleep 1
)
call :resolve_backend_port
if errorlevel 1 exit /b 1
set "_TAG=centag-%_DD%:latest"
if not defined LLM_PROXY_SYSTEM_PROXY_PORT set "LLM_PROXY_SYSTEM_PROXY_PORT=8081"
for %%V in (storage logs certs) do (
docker volume inspect "centag-%_DD%-%%V" >nul 2>&1
if errorlevel 1 docker volume create "centag-%_DD%-%%V" >nul
)
echo.
call :log_info "========================================"
call :log_info " Docker deployment"
call :log_info "  product edition: %_DD%"
call :log_info " --: http://localhost:%BACKEND_PORT%"
call :log_info "  press Ctrl+C to stop"
call :log_info "========================================"
echo.
docker run -it --rm --name "centag-%_DD%" --env-file "%PROJECT_ROOT%\config\secrets\.env" -e CENTAG_EDITION="%_DD%" -e CENTAG_IN_DOCKER=1 -e CENTAG_DATA_DIR=/app/storage -e LLM_PROXY_DB_DRIVER=sqlite -e SQLITE_PATH=/app/storage/centag.db -e MEMORY_STORE_ROOT=/app/storage/memory-store -e LLM_PROXY_LOG_OUTPUT=both -e LLM_PROXY_LOG_FORMAT=console -e LLM_PROXY_LOG_PATH=/app/logs -p "%BACKEND_PORT%:20060" -p "%LLM_PROXY_SYSTEM_PROXY_PORT%:8081" -v "centag-%_DD%-storage:/app/storage" -v "centag-%_DD%-logs:/app/logs" -v "centag-%_DD%-certs:/app/bin/certs" "%_TAG%"
exit /b %ERRORLEVEL%

:cmd_stop
set "SSVC=all"
set "SSVC_SET=0"
set "S_DOCKER=0"
:cmd_stop_parse
if "%~1"=="" goto cmd_stop_done
if /i "%~1"=="--docker" (
set "S_DOCKER=1"
shift
goto cmd_stop_parse
)
if "%SSVC_SET%"=="0" (
set "SSVC=%~1"
set "SSVC_SET=1"
shift
goto cmd_stop_parse
)
shift
goto cmd_stop_parse
:cmd_stop_done
if "%S_DOCKER%"=="1" (
set "_CT=centag-%SSVC%"
docker ps --format "{{.Names}}" 2>nul | findstr /B /E /C:"%_CT%" >nul 2>&1
if not errorlevel 1 (
call :log_info "- Docker -: %_CT%"
docker stop "%_CT%" >nul 2>&1
call :log_ok "  stopped container %_CT%"
exit /b 0
)
call :log_warn "  container %_CT% not running"
exit /b 0
)
call :normalize_type "%SSVC%"
set "SSVC=%_RET%"
if /i "%SSVC%"=="backend" (
call :stop_backend_only
exit /b !ERRORLEVEL!
)
if /i "%SSVC%"=="frontend" (
call :kill_port "%WEBUI_PORT%"
call :log_ok "Vue dev server stopped"
exit /b 0
)
if /i "%SSVC%"=="daemon" (
call :daemon_stop
exit /b !ERRORLEVEL!
)
if /i "%SSVC%"=="all" (
call :stop_all_services
set "_RC=!ERRORLEVEL!"
call :kill_port "%WEBUI_PORT%" >nul
call :log_ok "All services stopped"
exit /b !_RC!
)
if /i "%SSVC%"=="personal" goto cmd_stop_need_docker
if /i "%SSVC%"=="minimal" goto cmd_stop_need_docker
call :log_err "  unsupported service: %SSVC%"
call :log_info "-: be fe daemon all, -- personal --docker / minimal --docker"
exit /b 1

:cmd_stop_need_docker
call :log_err "  service %SSVC% requires --docker"
call :log_info "-: start.bat stop %SSVC% --docker"
exit /b 1

:cmd_status
call :print_title
echo.
call :log_raw "--:"
call :port_pid "%BACKEND_PORT%"
if defined _RET (
call :log_ok "  backend is running (port %BACKEND_PORT%, PID %_RET%)"
) else (
call :log_warn "  backend is not running"
)
echo.
call :log_raw "Vue dev server:"
call :port_pid "%WEBUI_PORT%"
if defined _RET (
    call :log_ok "  Vue dev server running (port %WEBUI_PORT%, PID %_RET%)"
) else (
    call :log_warn "  Vue dev server not running"
)
echo.
call :log_raw "-:"
call :daemon_status
echo.
call :log_info "--:"
call :log_info " - API: http://localhost:%BACKEND_PORT%"
call :log_info " Web -: http://localhost:%BACKEND_PORT%"
call :port_pid "%WEBUI_PORT%"
if defined _RET call :log_info " Vue Dev: http://localhost:%WEBUI_PORT%"
echo.
exit /b 0

:cmd_logs
call :port_pid "%BACKEND_PORT%"
if not defined _RET (
call :log_warn "  backend is not running"
exit /b 0
)
call :log_info "Backend PID: %_RET%"
set "_LDIR=%BIN_DIR%\logs"
if not exist "%_LDIR%" set "_LDIR=%BIN_DIR%\storage\logs"
call :log_info "--: %_LDIR%"
if not exist "%_LDIR%" (
    call :log_warn "log dir does not exist (service may not have produced logs yet)"
exit /b 0
)
dir /a /o-d "%_LDIR%"
if exist "%_LDIR%\centag.log" (
    call :log_info "live view: powershell Get-Content -Wait logfile"
)
exit /b 0

:cmd_clean
set "CTARGET=build"
set "CYES=0"
:cmd_clean_parse
if "%~1"=="" goto cmd_clean_done
if /i "%~1"=="-y" (
set "CYES=1"
shift
goto cmd_clean_parse
)
if /i "%~1"=="--yes" (
set "CYES=1"
shift
goto cmd_clean_parse
)
if /i "%~1"=="build" (
set "CTARGET=build"
shift
goto cmd_clean_parse
)
if /i "%~1"=="install" (
set "CTARGET=install"
shift
goto cmd_clean_parse
)
if /i "%~1"=="deploy" (
set "CTARGET=install"
shift
goto cmd_clean_parse
)
if /i "%~1"=="all" (
set "CTARGET=all"
shift
goto cmd_clean_parse
)
if /i "%~1"=="help" (
call :help_clean
exit /b 0
)
if /i "%~1"=="-h" (
call :help_clean
exit /b 0
)
if /i "%~1"=="--help" (
call :help_clean
exit /b 0
)
call :log_err "- clean -: %~1"
call :log_info "usage: start.bat clean build or install or deploy or all, add -y"
exit /b 1
:cmd_clean_done
if /i "%CTARGET%"=="build" (
call :clean_build "%CYES%"
exit /b !ERRORLEVEL!
)
if /i "%CTARGET%"=="install" (
call :clean_install "%CYES%"
exit /b !ERRORLEVEL!
)
call :clean_build "%CYES%"
call :clean_install "%CYES%"
call :clean_local_runtime "%CYES%"
exit /b 0

:clean_local_runtime
set "_CY=%~1"
set "_LR=%PROJECT_ROOT%\bin\server"
if not exist "%_LR%" (
call :log_warn "  launcher dir missing: %_LR%"
exit /b 0
)
call :log_warn "will delete in-repo local runtime dirs (binaries, static assets, DB and logs, and other runtime data):"
call :log_info " %_LR%"
if not "%_CY%"=="1" (
    call :confirm "confirm delete above dirs? (type yes to continue)" "n"
if errorlevel 1 (
call :log_warn "  operation cancelled"
exit /b 1
)
)
call :log_info "stopping local centag processes first..."
call :stop_all_services >nul 2>&1
rd /s /q "%_LR%" >nul 2>&1
call :log_ok "  removed launcher dir: %_LR%"
exit /b 0

:clean_build
set "_CY=%~1"
call :log_info "  cleaning build dir: %BIN_DIR%"
if not exist "%BIN_DIR%" (
call :log_warn "--, -: %BIN_DIR%"
exit /b 0
)
if not "%_CY%"=="1" (
    call :confirm "confirm delete build output dir? (type yes to continue)" "n"
if errorlevel 1 (
call :log_warn "  operation cancelled"
exit /b 1
)
)
call :daemon_stop >nul 2>&1
call :stop_backend_only >nul 2>&1
rd /s /q "%BIN_DIR%" >nul 2>&1
if exist "%CENTAG_BIN_DIR%\centag-%CENTAG_EDITION%.exe" del /q "%CENTAG_BIN_DIR%\centag-%CENTAG_EDITION%.exe" >nul 2>&1
call :log_ok "build output cleaned (re-run start.bat build if needed)"
exit /b 0

:clean_install
set "_CY=%~1"
set "_ROOT=%CENTAG_INSTALL_ROOT%"
if not defined _ROOT (
call :log_err "CENTAG_INSTALL_ROOT not set"
exit /b 1
)
if /i "%_ROOT%"=="%PROJECT_ROOT%" (
    call :log_err "refused to delete dangerous path: %_ROOT%"
exit /b 1
)
if /i "%_ROOT%"=="%USERPROFILE%" (
    call :log_err "refused to delete dangerous path: %_ROOT%"
exit /b 1
)
if "%_ROOT:~1%"==":\" (
    call :log_err "refused to delete drive root: %_ROOT%"
exit /b 1
)
set "_CHK=%_ROOT%"
call set "_CHK=%%_CHK:%PROJECT_ROOT%\=%%"
if not "%_CHK%"=="%_ROOT%" (
    call :log_err "refused to delete path inside repo: %_ROOT%"
exit /b 1
)
if not exist "%_ROOT%" (
call :log_warn "  directory missing: %_ROOT%"
exit /b 0
)
call :log_warn "will delete deployed / install layout (binaries, Web static, runtime DB and logs, release artifacts, etc.):"
call :log_info " %_ROOT%"
call :log_info "will not touch in-repo config\\secrets\\.env"
if not "%_CY%"=="1" (
    call :confirm "confirm delete above dirs? (type yes to continue)" "n"
if errorlevel 1 (
call :log_warn "  operation cancelled"
exit /b 1
)
)
call :log_info "stopping local centag processes first..."
call :stop_all_services >nul 2>&1
if exist "%CENTAG_BIN_DIR%\centag-wrap.exe" "%CENTAG_BIN_DIR%\centag-wrap.exe" disable >nul 2>&1
rd /s /q "%_ROOT%" >nul 2>&1
call :log_ok "  removed: %_ROOT%"
exit /b 0

:cmd_stack
set "_STACK_DIR=%PROJECT_ROOT%\deploy\stack"
if not exist "%_STACK_DIR%\start.sh" (
    call :log_err "deploy\\stack\\start.sh not found, run git submodule update --init"
exit /b 1
)
call :need_bash
if errorlevel 1 exit /b 1
set "STACK_ROOT=%_STACK_DIR%"
set "STACK_INVOKER=start.bat stack"
set "STACK_QUIET_CD=1"
if "%~1"=="" (
call :run_sh "%_STACK_DIR%\start.sh" help
exit /b !ERRORLEVEL!
)
call :run_sh "%_STACK_DIR%\start.sh" %1 %2 %3 %4 %5 %6 %7 %8 %9
exit /b !ERRORLEVEL!

:cmd_test
call :log_info "running tests..."
call :check_go
if errorlevel 1 exit /b 1
pushd "%PROJECT_ROOT%"
go test ./... -v -timeout=30s
set "_RC=%ERRORLEVEL%"
popd
if "%_RC%"=="0" (
    call :log_ok "all tests passed"
exit /b 0
)
call :log_err "some tests failed"
exit /b 1

rem ============================================================================
rem Docker
rem ============================================================================

:dist_docker_build
set "_DDB=%~1"
if not defined _DDB (
call :log_err "  edition required: minimal or personal"
exit /b 1
)
if /i "%_DDB%"=="team" (
call :log_err "Team Docker: use centag-pro instead"
exit /b 1
)
if /i not "%_DDB%"=="minimal" (
if /i not "%_DDB%"=="personal" (
call :log_err "  unsupported edition: %_DDB% (expected minimal or personal)"
exit /b 1
)
)
call :check_docker
if errorlevel 1 exit /b 1
set "_DDTAG=centag-%_DDB%:latest"
set "_DDTMP=%TEMP%\centag-initdata-%RANDOM%"
if not exist "%_DDTMP%" md "%_DDTMP%"
if not exist "%_DDTMP%\pipeline-templates\common" md "%_DDTMP%\pipeline-templates\common"
if exist "%PROJECT_ROOT%\config\initdata\pipeline-templates\common" (
copy /y "%PROJECT_ROOT%\config\initdata\pipeline-templates\common\*.yaml" "%_DDTMP%\pipeline-templates\common\" >nul 2>&1
)
(
echo version: "2.0"
echo description: First-boot empty backends - add providers in WebUI
echo backends: []
) > "%_DDTMP%\initial-backends.yaml"
call :dist_tags "%_DDB%"
set "_DDTAGS=%_RET%"
call :log_info "- Docker -: %_DDTAG% (dist=%_DDB%)..."
pushd "%PROJECT_ROOT%"
docker build --build-arg DIST_NAME="%_DDB%" --build-arg INCLUDE_FRONTEND=true --build-arg INITDATA_ARCHIVE=true --build-arg BUILD_TAGS="%_DDTAGS%" --build-context initdata="%_DDTMP%" -t "%_DDTAG%" -f deploy\docker\Dockerfile.dist .
set "_RC=%ERRORLEVEL%"
popd
rd /s /q "%_DDTMP%" >nul 2>&1
if "%_RC%"=="0" (
call :log_ok "  built image: %_DDTAG%"
exit /b 0
)
call :log_err "  docker build failed"
exit /b 1

rem %1=dist %2=port %3=initdata (kept arg) %4=reset
:dist_docker_run
set "_DDR=%~1"
set "_DDPORT=%~2"
if not defined _DDPORT set "_DDPORT=20060"
set "_DDRESET=%~4"
if not defined _DDRESET set "_DDRESET=false"
if /i "%_DDR%"=="team" (
call :log_err "Team Docker: use centag-pro instead"
exit /b 1
)
if /i not "%_DDR%"=="minimal" (
if /i not "%_DDR%"=="personal" (
call :log_err "  unsupported edition: %_DDR% (expected minimal or personal)"
exit /b 1
)
)
call :check_docker
if errorlevel 1 exit /b 1
set "_DDRTAG=centag-%_DDR%:latest"
docker image inspect "%_DDRTAG%" >nul 2>&1
if errorlevel 1 (
    call :log_info "image %_DDRTAG% not exist, building first..."
call :dist_docker_build "%_DDR%"
if errorlevel 1 exit /b 1
)
call :load_env
docker ps -a --format "{{.Names}}" 2>nul | findstr /B /E /C:"centag-%_DDR%" >nul 2>&1
if not errorlevel 1 (
call :log_info "  removing container centag-%_DDR%..."
docker rm -f "centag-%_DDR%" >nul 2>&1
call :sleep 1
)
call :resolve_backend_port
if errorlevel 1 exit /b 1
set "_DDPORT=%BACKEND_PORT%"
if not defined LLM_PROXY_SYSTEM_PROXY_PORT set "LLM_PROXY_SYSTEM_PROXY_PORT=8081"
if /i "%_DDRESET%"=="true" (
    call :log_warn "resetting %_DDR% data volume..."
for %%V in (storage logs certs) do docker volume rm -f "centag-%_DDR%-%%V" >nul 2>&1
)
for %%V in (storage logs certs) do (
docker volume inspect "centag-%_DDR%-%%V" >nul 2>&1
if errorlevel 1 docker volume create "centag-%_DDR%-%%V" >nul
)
call :log_info "data volumes: centag-%_DDR%-storage / -logs / -certs"
call :log_info "  note: centag-%_DDR% (port %_DDPORT%)"
docker run -d --rm --name "centag-%_DDR%" --env-file "%PROJECT_ROOT%\config\secrets\.env" -e CENTAG_EDITION="%_DDR%" -e CENTAG_IN_DOCKER=1 -e CENTAG_DATA_DIR=/app/storage -e LLM_PROXY_DB_DRIVER=sqlite -e SQLITE_PATH=/app/storage/centag.db -e MEMORY_STORE_ROOT=/app/storage/memory-store -e LLM_PROXY_LOG_OUTPUT=both -e LLM_PROXY_LOG_FORMAT=console -e LLM_PROXY_LOG_PATH=/app/logs -p "%_DDPORT%:20060" -p "%LLM_PROXY_SYSTEM_PROXY_PORT%:8081" -v "centag-%_DDR%-storage:/app/storage" -v "centag-%_DDR%-logs:/app/logs" -v "centag-%_DDR%-certs:/app/bin/certs" "%_DDRTAG%"
if errorlevel 1 (
call :log_err "  docker run failed"
exit /b 1
)
echo.
call :log_ok "  started container centag-%_DDR%"
call :log_info "--: docker logs -f centag-%_DDR%"
call :log_info "--: docker stop centag-%_DDR%"
exit /b 0

:docker_up
set "_DUE=%~1"
if not defined _DUE set "_DUE=personal"
if /i not "%_DUE%"=="personal" (
if /i not "%_DUE%"=="minimal" (
call :log_err "  unsupported edition: %_DUE% (expected personal or minimal)"
exit /b 1
)
)
call :check_docker
if errorlevel 1 exit /b 1
if not exist "%PROJECT_ROOT%\config\secrets\.env" (
    call :log_warn "config\\secrets\\.env not found, auto-generating auth config..."
set "_GSH2=%PROJECT_ROOT%\scripts\ops\generate-secrets.sh"
if not exist "%_GSH2%" set "_GSH2=%PROJECT_ROOT%\scripts\generate-secrets.sh"
if exist "%_GSH2%" call :run_sh "%_GSH2%" --same-password
)
call :load_env
set "_DUIMG=centag-%_DUE%:latest"
docker image inspect "%_DUIMG%" >nul 2>&1
if errorlevel 1 (
    call :log_warn "main service image %_DUIMG% not exist, building..."
call :dist_docker_build "%_DUE%"
if errorlevel 1 exit /b 1
)
call :compose_cmd
if errorlevel 1 (
call :log_err "  docker-compose not found"
exit /b 1
)
set "_DCCMD=%_RET%"
call :log_info "- Centag -: %_DUE%"
echo.
if not exist "%PROJECT_ROOT%\deploy\docker" (
call :log_err "--: %PROJECT_ROOT%\deploy\docker"
exit /b 1
)
pushd "%PROJECT_ROOT%\deploy\docker"
if exist "%PROJECT_ROOT%\config\secrets\.env" (
%_DCCMD% --env-file "%PROJECT_ROOT%\config\secrets\.env" up -d
) else (
%_DCCMD% up -d
)
set "_RC=%ERRORLEVEL%"
popd
if not "%_RC%"=="0" (
call :log_err "  docker compose up failed"
exit /b 1
)
call :log_ok "  done"
echo.
call :log_info "service info:"
call :log_info " - Centag: http://localhost:20060"
echo.
call :log_info "--: start.bat docker logs"
call :log_info "--: start.bat docker status"
call :log_info "--: start.bat docker down"
exit /b 0

:docker_down
call :check_docker
if errorlevel 1 exit /b 1
call :compose_cmd
if errorlevel 1 (
call :log_err "  docker-compose not found"
exit /b 1
)
set "_DCCMD=%_RET%"
call :log_info "- Docker Compose -..."
if not exist "%PROJECT_ROOT%\deploy\docker" (
call :log_err "--: %PROJECT_ROOT%\deploy\docker"
exit /b 1
)
pushd "%PROJECT_ROOT%\deploy\docker"
if exist "docker-compose.debug.host.yaml" (
if exist "%PROJECT_ROOT%\config\secrets\.env" (
%_DCCMD% --env-file "%PROJECT_ROOT%\config\secrets\.env" -f docker-compose.yaml -f docker-compose.debug.host.yaml down --remove-orphans >nul 2>&1
) else (
%_DCCMD% -f docker-compose.yaml -f docker-compose.debug.host.yaml down --remove-orphans >nul 2>&1
)
)
if exist "%PROJECT_ROOT%\config\secrets\.env" (
%_DCCMD% --env-file "%PROJECT_ROOT%\config\secrets\.env" down --remove-orphans
) else (
%_DCCMD% down --remove-orphans
)
set "_RC=%ERRORLEVEL%"
popd
if not "%_RC%"=="0" (
call :log_err "  docker compose down failed"
exit /b 1
)
call :log_ok "  done"
exit /b 0

rem  assemble compose extra args: _RET=config file; _RET2=env file
:docker_compose_extra
set "_RET="
if exist "%PROJECT_ROOT%\deploy\docker\docker-compose.debug.host.yaml" set "_RET=-f docker-compose.yaml -f docker-compose.debug.host.yaml"
set "_RET2="
if exist "%PROJECT_ROOT%\config\secrets\.env" set "_RET2=--env-file "%PROJECT_ROOT%\config\secrets\.env""
exit /b 0

:docker_logs
call :check_docker
if errorlevel 1 exit /b 1
call :compose_cmd
if errorlevel 1 (
call :log_err "  docker-compose not found"
exit /b 1
)
set "_DCCMD=%_RET%"
call :docker_compose_extra
if not exist "%PROJECT_ROOT%\deploy\docker" (
call :log_err "--: %PROJECT_ROOT%\deploy\docker"
exit /b 1
)
pushd "%PROJECT_ROOT%\deploy\docker"
%_DCCMD% %_RET2% %_RET% logs -f %1
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:docker_status
call :check_docker
if errorlevel 1 exit /b 1
call :compose_cmd
if errorlevel 1 (
call :log_err "  docker-compose not found"
exit /b 1
)
set "_DCCMD=%_RET%"
call :docker_compose_extra
call :log_info "Docker --:"
echo.
if not exist "%PROJECT_ROOT%\deploy\docker" (
call :log_err "--: %PROJECT_ROOT%\deploy\docker"
exit /b 1
)
pushd "%PROJECT_ROOT%\deploy\docker"
%_DCCMD% %_RET2% %_RET% ps
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:docker_restart
call :check_docker
if errorlevel 1 exit /b 1
call :compose_cmd
if errorlevel 1 (
call :log_err "  docker-compose not found"
exit /b 1
)
set "_DCCMD=%_RET%"
set "_DRS=%~1"
if not defined _DRS set "_DRS=centag"
call :docker_compose_extra
if not exist "%PROJECT_ROOT%\deploy\docker" (
call :log_err "--: %PROJECT_ROOT%\deploy\docker"
exit /b 1
)
pushd "%PROJECT_ROOT%\deploy\docker"
call :log_info "- %_DRS% ..."
%_DCCMD% %_RET2% %_RET% restart "%_DRS%"
set "_RC=%ERRORLEVEL%"
popd
if not "%_RC%"=="0" (
call :log_err "  docker restart failed"
exit /b 1
)
call :log_ok "  restarted service %_DRS%"
exit /b 0

:docker_clean
call :check_docker
if errorlevel 1 exit /b 1
call :compose_cmd
if errorlevel 1 (
call :log_err "  docker-compose not found"
exit /b 1
)
set "_DCCMD=%_RET%"
call :log_warn "will delete all containers, images, data volumes, confirm to continue?"
call :confirm "confirm continue" "n"
if errorlevel 1 (
    call :log_info "operation cancelled"
exit /b 0
)
call :docker_compose_extra
if not exist "%PROJECT_ROOT%\deploy\docker" exit /b 1
pushd "%PROJECT_ROOT%\deploy\docker"
%_DCCMD% %_RET2% %_RET% down -v --rmi all
popd
docker rmi centag-personal:latest >nul 2>&1
docker rmi centag-minimal:latest >nul 2>&1
call :log_ok "  done"
exit /b 0

:docker_pack
call :check_docker
if errorlevel 1 exit /b 1
call :log_info "- Docker -..."
call :resolve_timestamp
set "_TS=%_RET_STAMP%"
set "_PKGNAME=centag-docker-%_TS%"
set "_PKGDIR=%PROJECT_ROOT%\release\%_PKGNAME%"
docker image inspect centag-personal:latest >nul 2>&1
if errorlevel 1 (
call :log_warn "  image missing, building now..."
call :dist_docker_build personal
if errorlevel 1 exit /b 1
)
if not exist "%_PKGDIR%" md "%_PKGDIR%"
call :log_info "exporting main service image..."
docker save -o "%_PKGDIR%\centag-image.tar" centag-personal:latest
if errorlevel 1 (
call :log_err "  docker save failed"
exit /b 1
)
call :log_ok "  saved image: %_PKGDIR%\centag-image.tar"
exit /b 0

:cmd_docker
set "_DSUB=%~1"
if not defined _DSUB set "_DSUB=status"
if /i "%_DSUB%"=="build" goto cmd_docker_build
if /i "%_DSUB%"=="run" goto cmd_docker_run
if /i "%_DSUB%"=="up" (
call :docker_up %2 %3 %4
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="down" (
call :docker_down
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="logs" (
call :docker_logs %2 %3
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="status" (
call :docker_status
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="clean" (
call :docker_clean
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="pack" (
call :docker_pack
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="restart" (
call :docker_restart %2
exit /b !ERRORLEVEL!
)
if /i "%_DSUB%"=="debug" (
call :debug_docker %2
exit /b !ERRORLEVEL!
)
call :log_err "- docker -: %_DSUB%"
call :log_info "-: start.bat docker [-] [-]"
echo.
call :log_info "edition operations:"
call :log_info " start.bat docker build minimal - personal"
call :log_info " start.bat docker run minimal - personal [port] [--reset]"
echo.
call :log_info "Compose operations:"
call :log_info " start.bat docker up / down / logs / status / clean / pack / restart / debug"
exit /b 1

:cmd_docker_build
set "_DE=%~2"
if not defined _DE (
call :log_err "  edition required: minimal or personal (team uses centag-pro)"
call :log_info "-: start.bat docker build minimal - personal"
    call :log_info "compat aliases: all => personal; backend or be => minimal"
exit /b 1
)
if /i "%_DE%"=="all" set "_DE=personal"
if /i "%_DE%"=="backend" set "_DE=minimal"
if /i "%_DE%"=="be" set "_DE=minimal"
if /i "%_DE%"=="frontend" set "_DE=personal"
if /i "%_DE%"=="fe" set "_DE=personal"
call :dist_docker_build "%_DE%"
exit /b %ERRORLEVEL%

:cmd_docker_run
set "_DRE=%~2"
if not defined _DRE (
call :log_err "  edition required: minimal or personal"
call :log_info "-: start.bat docker run minimal - personal [port] [--reset]"
exit /b 1
)
set "_DRPORT=20060"
set "_DRRESET=false"
set "_DRSHIFT=0"
:docker_run_parse
if "%~1"=="" goto docker_run_done
if "%_DRSHIFT%"=="0" (
set "_DRSHIFT=1"
shift
goto docker_run_parse
)
if "%_DRSHIFT%"=="1" (
set "_DRSHIFT=2"
shift
goto docker_run_parse
)
if /i "%~1"=="--reset" (
set "_DRRESET=true"
shift
goto docker_run_parse
)
echo %~1 | findstr /R "^[0-9][0-9]*$" >nul
if not errorlevel 1 set "_DRPORT=%~1"
shift
goto docker_run_parse
:docker_run_done
call :dist_docker_run "%_DRE%" "%_DRPORT%" "" "%_DRRESET%"
exit /b %ERRORLEVEL%

rem ============================================================================
rem -
rem ============================================================================

:cmd_pack
call :log_info "hint: pack is merged into the package command, recommend start.bat package ota"
set "_PKGSH=%PROJECT_ROOT%\scripts\release\package.sh"
if not exist "%_PKGSH%" (
call :log_err "  script missing: %_PKGSH%"
exit /b 1
)
call :run_sh "%_PKGSH%" %1 %2 %3 %4 %5 %6 %7 %8 %9
exit /b %ERRORLEVEL%

:cmd_package
if /i "%~1"=="ota" goto cmd_package_ota
if /i "%~1"=="service" goto cmd_package_ota
if /i "%~1"=="update" goto cmd_package_ota
if /i "%~1"=="hot-update" goto cmd_package_ota
set "_PGSH=%PROJECT_ROOT%\scripts\packaging\package.sh"
if not exist "%_PGSH%" (
call :log_err "  script missing: %_PGSH%"
exit /b 1
)
call :run_sh "%_PGSH%" %1 %2 %3 %4 %5 %6 %7 %8 %9
exit /b %ERRORLEVEL%

:cmd_package_ota
set "_PKGSH2=%PROJECT_ROOT%\scripts\release\package.sh"
if not exist "%_PKGSH2%" (
call :log_err "  script missing: %_PKGSH2%"
exit /b 1
)
shift
call :run_sh "%_PKGSH2%" %1 %2 %3 %4 %5 %6 %7 %8 %9
exit /b %ERRORLEVEL%

rem ============================================================================
rem  wizard
rem ============================================================================

:wizard_step
echo.
call :log_raw "============================================================"
call :log_raw "  step %~1: %~2"
call :log_raw "============================================================"
echo.
exit /b 0

:wizard_env_config
call :wizard_step "2" "env"
call :log_raw "  configure environment (.env)"
echo.
if not exist "%PROJECT_ROOT%\config\secrets\.env" (
    call :log_warn "config\\secrets\\.env not exist (master key config missing)"
call :confirm "-----?" "y"
if not errorlevel 1 (
set "_GSH3=%PROJECT_ROOT%\scripts\generate-secrets.sh"
if not exist "%_GSH3%" set "_GSH3=%PROJECT_ROOT%\scripts\ops\generate-secrets.sh"
if exist "%_GSH3%" (
call :run_sh "%_GSH3%" --same-password
) else (
            call :log_err "generate-secrets script not found"
)
)
) else (
    call :log_info "config\\secrets\\.env already exists, skip generation (re-run start.bat env gen to regenerate)"
)
if not exist "%BIN_DIR%" (
    call :log_warn "bin dir not exist, need to initialize"
call :confirm "-- setup --?" "y"
if not errorlevel 1 call :cmd_init
)
exit /b 0

:wizard_check_pg
call :wizard_step "4" "PostgreSQL"
call :log_raw "  verify PostgreSQL connection"
echo.
set "_PGRUN=0"
call :has_cmd docker
if "%_RET%"=="1" (
docker ps --format "{{.Names}}" 2>nul | findstr /I "postgres" >nul 2>&1
if not errorlevel 1 set "_PGRUN=1"
)
if "%_PGRUN%"=="1" (
    call :log_ok "detected running PostgreSQL-related container, no action needed"
exit /b 0
)
call :log_warn "no obvious PostgreSQL container name detected in docker ps"
echo.
call :log_raw "PostgreSQL and Mem0 middleware have moved to subproject deploy\\stack, e.g.:"
call :log_info "    cd deploy\\stack then start.bat start base"
echo.
call :confirm "I already started the database in another terminal, continue wizard" "y"
if not errorlevel 1 exit /b 0
call :log_warn "start the database first, then run this wizard or start.bat run be"
exit /b 0

:wizard_build
call :wizard_step "3" "build"
call :log_raw "build backend service and frontend assets"
echo.
set "_HASVUE=0"
if exist "%PROJECT_ROOT%\web" set "_HASVUE=1"
call :log_raw "select services to build:"
call :log_info " 1 / all   - build all (includes Vue)"
call :log_info " 2 / be   - backend service"
if "%_HASVUE%"=="1" (
call :log_info " 3 / vue - -- Vue -"
call :log_info " 4 / skip"
) else (
call :log_info " 3 / skip"
)
echo.
call :log_warn "hint: you can type a number or shorthand (e.g. 1 or all)"
echo.
call :wizard_read "---" "1"
set "_WBC=%_RET%"
if /i "%_WBC%"=="1" goto wb_all
if /i "%_WBC%"=="all" goto wb_all
if /i "%_WBC%"=="2" goto wb_be
if /i "%_WBC%"=="be" goto wb_be
if /i "%_WBC%"=="backend" goto wb_be
if /i "%_WBC%"=="3" goto wb_vue
if /i "%_WBC%"=="vue" goto wb_vue
goto wb_skip

:wb_all
call :log_raw "starting full build (includes Vue frontend)..."
call :build_all_targets all
set "WIZARD_VUE_BUILT=1"
exit /b 0

:wb_be
call :log_raw "  building backend..."
call :build_all_targets backend
if "%_HASVUE%"=="1" (
echo.
    call :confirm "build Vue frontend as well?" "n"
if not errorlevel 1 (
call :build_all_targets webui
set "WIZARD_VUE_BUILT=1"
)
)
exit /b 0

:wb_vue
if "%_HASVUE%"=="1" (
call :log_raw "- Vue -..."
call :build_all_targets webui
set "WIZARD_VUE_BUILT=1"
) else (
    call :log_warn "skip build step"
)
exit /b 0

:wb_skip
call :log_warn "skip build step"
exit /b 0

:wizard_run_mode
call :wizard_step "5" "run"
call :log_raw "select how to run the service"
echo.
set "_HASVUE2=0"
if exist "%PROJECT_ROOT%\web" set "_HASVUE2=1"
call :log_raw "--:"
call :log_info "  1 / bg     - run in background (recommended for daily use)"
call :log_info "  2 / debug  - foreground debug mode (view live logs)"
call :log_info " 3 / daemon - background daemon"
call :log_info " 4 / docker - Docker container"
call :log_info " 5 / skip"
echo.
call :wizard_read "---" "1"
set "_WRM=%_RET%"
if /i "%_WRM%"=="5" exit /b 0
if /i "%_WRM%"=="skip" exit /b 0
if /i "%_WRM%"=="4" goto wrm_docker
if /i "%_WRM%"=="docker" goto wrm_docker
echo.
call :log_raw "select services to run:"
call :log_info " 1 / all   - build all (includes backend)"
call :log_info " 2 / be   - backend service"
if "%_HASVUE2%"=="1" (
    call :log_info "  3 / dev   - dev mode (backend + Vue dev server)"
    call :log_info "  4 / vue   - start Vue dev server only"
)
echo.
call :wizard_read "---" "1"
set "_WRC=%_RET%"
if /i "%_WRM%"=="1" goto wrm_bg
if /i "%_WRM%"=="bg" goto wrm_bg
if /i "%_WRM%"=="2" goto wrm_debug
if /i "%_WRM%"=="debug" goto wrm_debug
if /i "%_WRM%"=="3" goto wrm_daemon
if /i "%_WRM%"=="daemon" goto wrm_daemon
exit /b 0

:wrm_docker
call :log_raw "Docker..."
echo.
call :log_info "  Docker Compose"
echo.
call :docker_up personal
exit /b 0

:wrm_bg
call :log_raw "  run in background mode..."
echo.
if /i "%_WRC%"=="1" goto wbg_all
if /i "%_WRC%"=="all" goto wbg_all
if /i "%_WRC%"=="2" goto wbg_be
if /i "%_WRC%"=="be" goto wbg_be
if /i "%_WRC%"=="backend" goto wbg_be
if /i "%_WRC%"=="3" goto wbg_dev_hint
if /i "%_WRC%"=="dev" goto wbg_dev_hint
if /i "%_WRC%"=="4" goto wbg_vue
if /i "%_WRC%"=="vue" goto wbg_vue
exit /b 0

:wbg_all
call :log_info "  starting backend (port %BACKEND_PORT%)..."
call :confirm "start now?" "y"
if errorlevel 1 exit /b 0
call :run_all_dev
exit /b 0

:wbg_be
call :log_info "  starting backend (port %BACKEND_PORT%)..."
call :confirm "start now?" "y"
if errorlevel 1 exit /b 0
call :load_env
call :resolve_backend_port
if errorlevel 1 exit /b 0
if not exist "%BIN_DIR%\%SERVER_BIN%" call :build_backend
if not exist "%BIN_DIR%\logs" md "%BIN_DIR%\logs"
set "_BEBAT2=%BIN_DIR%\storage\centag-backend.bat"
set "_BGCMD="%BIN_DIR%\%SERVER_BIN%" serve >> "%BIN_DIR%\logs\centag.log" 2>&1"
call :write_bg_bat "%_BEBAT2%" "%BIN_DIR%"
start "centag-backend" /B cmd /c "%_BEBAT2%"
call :sleep 3
call :port_pid "%BACKEND_PORT%"
if defined _RET (
call :log_ok "  backend running (port %BACKEND_PORT%, PID %_RET%)"
call :log_info "--: %BIN_DIR%\logs\centag.log"
call :log_info "--: start.bat stop backend"
) else (
call :log_err "  backend failed to start"
)
exit /b 0

:wbg_vue
if "%_HASVUE2%"=="1" (
    call :log_info "starting Vue dev server... (http://localhost:%WEBUI_PORT%)"
    call :confirm "start now?" "y"
if errorlevel 1 exit /b 0
call :webui_dev
) else (
call :log_warn "  Vue dev server not started"
)
exit /b 0

:wbg_dev_hint
if "%_HASVUE2%"=="1" (
    call :log_info "dev mode: backend + Vue dev server..."
    call :log_warn "dev mode needs two terminal windows:"
    call :log_warn "     terminal1: start.bat run backend"
    call :log_warn "     terminal2: start.bat run frontend"
    call :confirm "start backend in current terminal now?" "y"
if errorlevel 1 exit /b 0
call :run_backend_fg
) else (
call :log_warn "  Vue dev server not started"
)
exit /b 0

:wrm_debug
call :log_raw "foreground debug mode..."
echo.
if /i "%_WRC%"=="1" goto wdb_be
if /i "%_WRC%"=="all" goto wdb_be
if /i "%_WRC%"=="2" goto wdb_be
if /i "%_WRC%"=="be" goto wdb_be
if /i "%_WRC%"=="backend" goto wdb_be
if /i "%_WRC%"=="4" goto wbg_vue
if /i "%_WRC%"=="vue" goto wbg_vue
exit /b 0

:wdb_be
call :log_info "  starting backend (port %BACKEND_PORT%)..."
call :confirm "start now?" "y"
if errorlevel 1 exit /b 0
call :load_env
call :debug_console_env
call :resolve_backend_port
if errorlevel 1 exit /b 0
if not exist "%BIN_DIR%\%SERVER_BIN%" call :build_backend
call :print_test_examples
pushd "%BIN_DIR%"
call :log_info "  press Ctrl+C to stop"
"%SERVER_BIN%" serve
set "_RC=%ERRORLEVEL%"
popd
exit /b %_RC%

:wrm_daemon
call :log_raw "  starting daemon..."
echo.
if /i "%_WRC%"=="1" goto wdm_all
if /i "%_WRC%"=="all" goto wdm_all
if /i "%_WRC%"=="2" goto wdm_be
if /i "%_WRC%"=="be" goto wdm_be
if /i "%_WRC%"=="backend" goto wdm_be
if /i "%_WRC%"=="3" (
    call :log_warn "daemon mode does not support dev mode"
exit /b 0
)
exit /b 0

:wdm_all
call :log_info "  starting backend (port %BACKEND_PORT%)..."
call :confirm "start now?" "y"
if errorlevel 1 exit /b 0
if not "%WIZARD_VUE_BUILT%"=="1" call :webui_build >nul 2>&1
call :daemon_start
exit /b 0

:wdm_be
call :log_info "  starting backend (port %BACKEND_PORT%)..."
call :confirm "start now?" "y"
if errorlevel 1 exit /b 0
call :daemon_start
exit /b 0

:wizard_finish
call :wizard_step "-" "finish"
call :log_ok "  wizard complete"
echo.
call :log_raw "Web UI login info:"
echo.
set "_SECF=%PROJECT_ROOT%\config\secrets\.env"
if not exist "%_SECF%" set "_SECF=%PROJECT_ROOT%\config\secrets\.env.middleware"
set "_ADU="
set "_ADP="
if exist "%_SECF%" (
call :read_env_value "%_SECF%" "LLM_PROXY_ADMIN_USERNAME"
set "_ADU=%_RET%"
call :read_env_value "%_SECF%" "LLM_PROXY_ADMIN_PASSWORD"
set "_ADP=%_RET%"
)
if defined _ADU (
if defined _ADP (
        call :log_ok "  username: %_ADU%"
call :log_ok " -: %_ADP%"
echo.
        call :log_warn "hint: change password after first login"
) else (
        call :log_warn "Web UI admin credential config not found"
        call :log_info "default username: admin"
        call :log_info "no default password preset; set it via the first-start init wizard"
        call :log_info "or run start.bat env gen to generate new credentials"
)
) else (
    call :log_warn "Web UI admin credential config not found"
    call :log_info "default username: admin"
    call :log_info "no default password preset; set it via the first-start init wizard"
    call :log_info "or run start.bat env gen to generate new credentials"
)
echo.
call :log_raw "common commands reference:"
echo.
call :log_info "--: start.bat status"
call :log_info "--: start.bat run backend"
call :log_info "dev mode:  start.bat debug"
call :log_info "--: start.bat logs"
call :log_info "--: start.bat stop all"
call :log_info "--: start.bat build all"
echo.
call :log_raw "  current status:"
call :port_pid "%BACKEND_PORT%"
if defined _RET (
call :log_ok " - API: http://localhost:%BACKEND_PORT%"
call :log_ok " Web -: http://localhost:%BACKEND_PORT%"
) else (
call :log_warn "  API: http://localhost:%BACKEND_PORT% (default)"
)
call :port_pid "%WEBUI_PORT%"
if defined _RET call :log_ok " Vue Dev: http://localhost:%WEBUI_PORT%"
echo.
call :log_ok "happy using"
exit /b 0

:cmd_wizard
set "WIZARD_VUE_BUILT=0"
call :print_title
echo.
call :log_raw "welcome to the project init wizard"
call :log_raw "this wizard will guide you through project init, build and deploy"
echo.
call :wizard_step "1" "--"
call :check_dependencies
call :wizard_env_config
call :wizard_build
call :wizard_check_pg
call :wizard_run_mode
call :wizard_finish
exit /b 0

rem ============================================================================
rem  help / version
rem ============================================================================

:print_title
call :log_raw "================================"
call :log_raw " Centag Manager"
call :log_raw "================================"
exit /b 0

:print_test_examples
echo.
call :log_info "========================================"
call :log_info "test script example"
call :log_info "========================================"
echo.
call :log_info "1. test chat request (non-streaming):"
call :log_info " curl -X POST http://localhost:%BACKEND_PORT%/v1/chat/completions -H ""Content-Type: application/json"" -d {\""model\"":\""qwen2.5:1.5b\"",\""messages\"":[{\""role\"":\""user\"",\""content\"":\""hello\""}],\""stream\"":false}"
echo.
call :log_info "2. health check:"
call :log_info " curl http://localhost:%BACKEND_PORT%/health"
echo.
exit /b 0

:show_version
echo.
call :log_raw "+------------------------------------------+"
call :log_raw " Centag"
call :log_raw "+------------------------------------------+"
call :log_raw " -: %CENTAG_VERSION%"
call :log_raw "  build no:    %VERSION%"
call :log_raw "  build time:   %BUILD_TIME%"
call :has_cmd go
if "%_RET%"=="1" (
set "_VGV="
for /f "tokens=3" %%g in ('go version 2^>nul') do set "_VGV=%%g"
call :log_raw " Go: !_VGV!"
)
call :log_raw "+------------------------------------------+"
echo.
call :log_info "run start.bat --help to view command list"
echo.
exit /b 0

:show_short_help
echo.
call :log_raw "+----------------------------------------------+"
call :log_raw " Centag Manager (Windows)"
call :log_raw " -: %CENTAG_VERSION%"
call :log_raw "+----------------------------------------------+"
echo.
call :log_ok "-: start.bat [-] [-...]"
echo.
call :log_warn "* quick start (newbies read here first)"
echo.
call :log_ok "  wizard       interactive init wizard (dep check / build / start in one go)"
call :log_ok "  debug        dev mode (foreground + live logs + frontend hot reload)"
call :log_ok "  docker up    Docker one-click deploy (default personal edition)"
call :log_ok "  status       show running service status"
call :log_ok "  logs         tail backend and daemon logs"
call :log_ok "  stop         stop all running services"
echo.
call :log_raw "  advanced commands (start.bat [command] --help for details)"
echo.
call :log_ok "  build       all / be / fe / personal / minimal  (add -y to skip prompts)"
call :log_ok "  build        --desktop / --docker / --wrap         desktop / Docker / system proxy"
echo.
call :log_ok "  run be / fe / all / personal / minimal (or up)""
call :log_ok " run wrap --background / --desktop / --docker"
echo.
call :log_ok "  daemon       backend / stop / debug / status       background daemon management"
call :log_ok "  clean       build / install / all  (add -y to skip prompts)"
call :log_ok "  stack        start / stop / status ...             middleware orchestration (PG/Redis/ES/...)"
call :log_ok "  docker       build / run / up / down / logs ...    container lifecycle and image build"
call :log_ok "  package      ota / cli / desktop / fnos ...        package OTA / CLI / desktop deploy bundle"
call :log_ok "  pack        build OTA / CLI / desktop / fnos packages"
call :log_ok "  webui        dev / build / lint / clean            Vue frontend dev"
call :log_ok "  init                                              init dev environment"
call :log_ok "  env         gen / show  generate or print .env config"
call :log_ok "  test                                              run unit tests"
echo.
call :log_raw "  alias quick reference"
call :log_ok " up - run dev - debug w / -w / --wizard - wizard"
echo.
call :log_raw "  about the team edition"
call :log_warn "    the team commercial edition is only built in the private repo centag-pro; this repo only supports personal and minimal."
echo.
call :log_warn "hint: start.bat [command] --help   view detailed command usage"
call :log_warn "      start.bat --version       view version info"
echo.
exit /b 0

:show_command_help
set "_HC=%~1"
echo.
if /i "%_HC%"=="wizard" ( call :help_wizard & exit /b 0 )
if /i "%_HC%"=="w" ( call :help_wizard & exit /b 0 )
if /i "%_HC%"=="-w" ( call :help_wizard & exit /b 0 )
if /i "%_HC%"=="--wizard" ( call :help_wizard & exit /b 0 )
if /i "%_HC%"=="init" ( call :help_init & exit /b 0 )
if /i "%_HC%"=="build" ( call :help_build & exit /b 0 )
if /i "%_HC%"=="run" ( call :help_run & exit /b 0 )
if /i "%_HC%"=="up" ( call :help_run & exit /b 0 )
if /i "%_HC%"=="daemon" ( call :help_daemon & exit /b 0 )
if /i "%_HC%"=="debug" ( call :help_debug & exit /b 0 )
if /i "%_HC%"=="dev" ( call :help_debug & exit /b 0 )
if /i "%_HC%"=="stop" ( call :help_stop & exit /b 0 )
if /i "%_HC%"=="status" ( call :help_status & exit /b 0 )
if /i "%_HC%"=="logs" ( call :help_logs & exit /b 0 )
if /i "%_HC%"=="clean" ( call :help_clean & exit /b 0 )
if /i "%_HC%"=="stack" ( call :help_stack & exit /b 0 )
if /i "%_HC%"=="docker" ( call :help_docker & exit /b 0 )
if /i "%_HC%"=="webui" ( call :help_webui & exit /b 0 )
if /i "%_HC%"=="pack" ( call :help_pack & exit /b 0 )
if /i "%_HC%"=="package" ( call :help_package & exit /b 0 )
if /i "%_HC%"=="test" ( call :help_test & exit /b 0 )
if /i "%_HC%"=="env" ( call :help_env & exit /b 0 )
call :log_err "--: %_HC%"
echo.
call :show_short_help
exit /b 1

:help_wizard
call :log_ok "-: wizard / w / -w"
call :log_warn "       interactive init wizard"
echo.
call :log_raw "-:"
call :log_info " start.bat wizard"
call :log_info " start.bat w"
echo.
call :log_raw "description:"
call :log_info "  guides through env config, project build, DB check and service start; for first-time use."
echo.
call :log_raw "wizard flow:"
call :log_info " 1. check dependencies (Go / Docker / Node.js)"
call :log_info " 2. configure environment (.env)"
call :log_info " 3. build services"
call :log_info " 4. configure PostgreSQL"
call :log_info " 5. run services"
exit /b 0

:help_init
call :log_ok "-: init"
call :log_warn "       init dev environment"
echo.
call :log_raw "-:"
call :log_info " start.bat init"
echo.
call :log_raw "description:"
call :log_info "  installs Go toolchain, configures environment, copies files."
call :log_info "  equivalent to start.sh's setup plus make copy-files."
exit /b 0

:help_build
call :log_ok "-: build"
call :log_warn "  usage: start.bat build <target>"
echo.
call :log_raw "-:"
call :log_info " start.bat build [-] [-]"
echo.
call :log_raw "-:"
call :log_info "  all              build all services"
call :log_info "  be / backend     build backend only"
call :log_info "  fe / frontend    build frontend only"
call :log_info "  personal         personal edition (full-feature tags)"
call :log_info "  minimal          minimal edition (full-feature tags)"
call :log_info " wrap -- centag-wrap"
echo.
call :log_raw "-:"
call :log_info "  --desktop        also build desktop shell (personal / minimal only)"
call :log_info "  --docker          Docker image (personal / minimal only)"
call :log_info "  --wrap           also build centag-wrap"
echo.
call :log_raw "example:"
call :log_info " start.bat build all"
call :log_info " start.bat build personal --desktop"
call :log_info " start.bat build minimal --docker"
exit /b 0

:help_run
call :log_ok "-: run (alias: up)"
call :log_warn "  usage: start.bat run <service>"
echo.
call :log_raw "-:"
call :log_info " start.bat run [-] [-]"
echo.
call :log_raw "-:"
call :log_info "  be / backend     start dev backend in foreground (default)"
call :log_info "  fe / frontend    start Vue dev server (5173)"
call :log_info "  all              backend background + frontend dev server"
call :log_info "  personal          personal edition"
call :log_info "  minimal           minimal edition"
call :log_info " wrap - centag-wrap"
echo.
call :log_raw "-:"
call :log_info "  --background / -b   run in background"
call :log_info "  --desktop           desktop tray form"
call :log_info "  --docker           Docker container form"
echo.
call :log_raw "example:"
call :log_info " start.bat run be"
call :log_info " start.bat run personal --background"
call :log_info " start.bat run minimal --desktop"
exit /b 0

:help_daemon
call :log_ok "-: daemon"
call :log_warn "       background daemon management"
echo.
call :log_raw "-:"
call :log_info " start.bat daemon backend / stop / debug / status"
echo.
call :log_raw "-:"
call :log_info "  backend   start daemon (default), auto-relaunch after crash"
call :log_info "  stop      stop daemon and its child processes"
call :log_info "  debug     start backend in foreground debug mode"
call :log_info "  status      show daemon status"
echo.
call :log_raw "description:"
call :log_info "  on Windows the supervisor runs in a minimized window, checking the port every %DAEMON_CHECK_INTERVAL% seconds."
call :log_info " PID -: %BIN_DIR%\storage\centag.daemon.pid"
call :log_info " --: %BIN_DIR%\logs\centag.log"
exit /b 0

:help_debug
call :log_ok "-: debug (dev mode)"
call :log_warn "       dev mode (frontend hot reload + backend live logs)"
echo.
call :log_raw "-:"
call :log_info "  start.bat debug personal or minimal, add --desktop / --docker"
echo.
call :log_raw "-:"
call :log_info "  edition=personal, form=cli (foreground sidecar)"
echo.
call :log_raw "example:"
call :log_info "  start.bat debug personal + --watch"
call :log_info " start.bat debug minimal - WebUI + centag-minimal"
call :log_info "  start.bat debug personal --desktop    tray form"
call :log_info "  start.bat debug personal --docker     foreground container"
echo.
call :log_warn "  use --watch to rebuild on file change"
exit /b 0

:help_stop
call :log_ok "-: stop"
call :log_warn "  usage: start.bat stop <service>"
echo.
call :log_raw "-:"
call :log_info " start.bat stop [-] [--docker]"
echo.
call :log_raw "-:"
call :log_info "  all   stop all services"
call :log_info "  be / backend   stop backend service only"
call :log_info "  fe / frontend    stop Vue dev server only"
call :log_info "  daemon        stop background daemon"
call :log_info "  personal --docker   personal edition (Docker)"
call :log_info "  minimal --docker    minimal edition (Docker)"
exit /b 0

:help_status
call :log_ok "-: status"
call :log_warn "  usage: start.bat status"
echo.
call :log_raw "-:"
call :log_info " start.bat status"
echo.
call :log_raw "checks:"
call :log_info "  checks backend port %BACKEND_PORT%"
call :log_info "  Vue dev server (port %WEBUI_PORT%)"
call :log_info "  PID file: %BIN_DIR%\storage\centag.daemon.pid"
exit /b 0

:help_logs
call :log_ok "-: logs"
call :log_warn "  usage: start.bat logs"
echo.
call :log_raw "-:"
call :log_info " start.bat logs"
echo.
call :log_raw "description:"
call :log_info "  list log dir contents."
call :log_info " --: %BIN_DIR%\logs"
call :log_info "  live tail: powershell Get-Content -Wait logfile"
exit /b 0

:help_clean
call :log_ok "-: clean"
call :log_warn "  usage: start.bat clean <build|install|all>"
echo.
call :log_raw "-:"
call :log_info "  start.bat clean build / install / deploy / all, add -y or --yes"
echo.
call :log_raw "-:"
call :log_info "  build edition - lib\%CENTAG_EDITION%"
call :log_info "  install      install to %CENTAG_INSTALL_ROOT%"
call :log_info "  deploy     same as install"
call :log_info "  all   build + install + copy to bin\server"
echo.
call :log_warn "  dangerous ops ask for confirmation twice; add -y in non-interactive env."
exit /b 0

:help_stack
call :log_ok "-: stack"
call :log_warn "       middleware orchestration (PG / Redis / ES / Qdrant / Mem0 etc.)"
echo.
call :log_raw "-:"
call :log_info " start.bat stack start / stop / status [-]"
echo.
call :log_raw "example:"
call :log_info " start.bat stack start base"
call :log_info " start.bat stack status"
echo.
call :log_warn "  deploy/stack/start.sh (git submodule) requires bash"
exit /b 0

:help_docker
call :log_ok "-: docker"
call :log_warn "       container lifecycle / image build / deploy bundle"
echo.
call :log_raw "edition operations:"
call :log_info " start.bat docker build minimal - personal"
call :log_info " start.bat docker run minimal - personal [port] [--reset]"
echo.
call :log_raw "Compose operations:"
call :log_info "  start.bat docker up personal - minimal (personal only)"
call :log_info "  start.bat docker down"
call :log_info "  start.bat docker logs    [service]               follow logs"
call :log_info "  start.bat docker status"
call :log_info "  start.bat docker restart [service]"
call :log_info "  start.bat docker debug                           debug mode (foreground container)"
call :log_info "  start.bat docker clean                           delete containers / images / data volumes"
call :log_info " start.bat docker pack -- tar"
exit /b 0

:help_webui
call :log_ok "-: webui"
call :log_warn "       Vue frontend dev"
echo.
call :log_raw "-:"
call :log_info " start.bat webui dev / build / lint / clean"
echo.
call :log_raw "-:"
call :log_info "  dev     start Vite dev server (http://localhost:%WEBUI_PORT%)"
call :log_info "  build   build static assets"
call :log_info " lint ESLint -"
call :log_info "  clean   delete static assets dir"
echo.
call :log_warn "  WebUI needs Node.js 20.19.0 or 22.12.0+"
exit /b 0

:help_pack
call :log_ok "-: pack"
call :log_warn "  usage: start.bat pack <ota|cli|desktop|fnos>"
echo.
call :log_raw "-:"
call :log_info " start.bat pack [-...]"
call :log_info " start.bat package ota [-...]"
echo.
call :log_warn "  this command delegates to scripts\\release\\package.sh (needs bash)."
exit /b 0

:help_package
call :log_ok "-: package"
call :log_warn "       unified deploy-bundle entry (OTA / CLI / desktop / fnOS / Docker)"
echo.
call :log_raw "-:"
call :log_info "  start.bat package ota [options]     server-side OTA update bundle"
call :log_info " start.bat package cli / desktop / fnos / docker [...]"
echo.
call :log_raw "example:"
call :log_info " start.bat package ota"
call :log_info " start.bat package ota --platforms windows-amd64,linux-amd64"
echo.
call :log_warn "  non-ota targets delegate to scripts\\packaging\\package.sh (needs bash)."
exit /b 0

:help_test
call :log_ok "-: test"
call :log_warn "       run unit tests"
echo.
call :log_raw "-:"
call :log_info " start.bat test"
echo.
call :log_raw "description:"
call :log_info "  equivalent to go test ./... -v -timeout=30s"
exit /b 0

:help_env
call :log_ok "-: env"
call :log_warn "  usage: start.bat env <gen|show>"
echo.
call :log_raw "-:"
call :log_info " start.bat env gen [--interactive] [--force]"
echo.
call :log_raw "description:"
call :log_info "  generate config\\secrets\\.env (main service secrets and PG meta DB)."
call :log_warn "  delegates to scripts\\generate-secrets.sh (needs bash)."
exit /b 0

rem ============================================================================
rem -
rem ============================================================================
:_exit0
endlocal
exit /b 0

:_exit1
endlocal
exit /b 1

:_exit_rc
set "_RC=%ERRORLEVEL%"
endlocal & exit /b %_RC%
