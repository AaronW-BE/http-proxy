@echo off

REM Default values
SET DEFAULT_NAME=go-proxy
SET DEFAULT_OS=windows

REM Parameters:
REM %1: Output name (optional, defaults to DEFAULT_NAME)
REM %2: Target OS (optional, 'linux' or 'windows', defaults to DEFAULT_OS)

SET OUTPUT_NAME=%1
IF "%OUTPUT_NAME%"=="" (
  SET OUTPUT_NAME=%DEFAULT_NAME%
)

SET TARGET_OS=%2
IF "%TARGET_OS%"=="" (
  SET TARGET_OS=%DEFAULT_OS%
)

REM Set Go environment variables for cross-compilation
SET GOARCH=amd64

echo Building Go proxy server...
echo Target OS: %TARGET_OS%
echo Output Name (base): %OUTPUT_NAME%

SET FINAL_OUTPUT_NAME=%OUTPUT_NAME%

IF /I "%TARGET_OS%"=="windows" (
  SET GOOS=windows
  REM Ensure .exe extension for Windows
  IF NOT "%FINAL_OUTPUT_NAME:~-4%"==".exe" (
    SET FINAL_OUTPUT_NAME=%FINAL_OUTPUT_NAME%.exe
  )
  echo Building for Windows: %FINAL_OUTPUT_NAME%
) ELSE IF /I "%TARGET_OS%"=="linux" (
  SET GOOS=linux
  REM Remove .exe extension if present for Linux
  IF "%FINAL_OUTPUT_NAME:~-4%"==".exe" (
    SET FINAL_OUTPUT_NAME=%FINAL_OUTPUT_NAME:~0,-4%
  )
  echo Building for Linux: %FINAL_OUTPUT_NAME%
) ELSE (
  echo Error: Unsupported target OS "%TARGET_OS%". Supported: 'linux', 'windows'.
  exit /b 1
)

go build -o "%FINAL_OUTPUT_NAME%" main.go

IF ERRORLEVEL 1 (
  echo Build failed.
  exit /b 1
) ELSE (
  echo Build successful! Output: %FINAL_OUTPUT_NAME%
  echo Done.
)
