@echo off

REM Default output name
SET DEFAULT_NAME=go-proxy

REM Use the first argument as the output name if provided, otherwise use the default
SET OUTPUT_NAME=%1
IF "%OUTPUT_NAME%"=="" (
  SET OUTPUT_NAME=%DEFAULT_NAME%
)

REM Ensure the output name has a .exe extension
IF NOT "%OUTPUT_NAME:~-4%"==".exe" (
  SET OUTPUT_NAME=%OUTPUT_NAME%.exe
)

echo Building Go proxy server for Windows...
go build -o "%OUTPUT_NAME%" main.go

IF ERRORLEVEL 1 (
  echo Build failed.
  exit /b 1
) ELSE (
  echo Build successful! Output: %OUTPUT_NAME%
  echo Done.
)
