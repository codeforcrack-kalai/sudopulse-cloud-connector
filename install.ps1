# SudoPulse Cloud Connector Agent Installation Script (Windows)

param (
    [Parameter(Mandatory=$true)]
    [string]$Token
)

$ErrorActionPreference = "Stop"

$InstallDir = "C:\Program Files\SudoPulse\Connector"
$BinPath = Join-Path $InstallDir "sudopulse-connector.exe"
$BinaryUrl = "https://github.com/sudopulse/connector/releases/latest/download/sudopulse-connector-windows-amd64.exe"
$ServiceName = "SudoPulseConnector"

Write-Host "Creating installation directory: $InstallDir"
if (-Not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

Write-Host "Downloading SudoPulse Connector..."
try {
    Invoke-WebRequest -Uri $BinaryUrl -OutFile $BinPath
} catch {
    Write-Error "Failed to download binary from $BinaryUrl"
    exit 1
}

# Check if service already exists
$Service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue

if ($Service) {
    Write-Host "Service $ServiceName already exists. Stopping and updating..."
    Stop-Service -Name $ServiceName -Force
} else {
    Write-Host "Registering SudoPulse Connector as a Windows Service..."
    New-Service -Name $ServiceName -BinaryPathName $BinPath -DisplayName "SudoPulse Cloud Connector" -Description "Tunnels traffic back to the SudoPulse Gateway" -StartupType Automatic | Out-Null
}

Write-Host "Configuring service environment variables..."
# Windows services need environment variables set in the registry
$RegistryPath = "HKLM:\SYSTEM\CurrentControlSet\Services\$ServiceName"
Set-ItemProperty -Path $RegistryPath -Name "Environment" -Value @("SUDOPULSE_TOKEN=$Token") -Type MultiString

Write-Host "Starting the service..."
Start-Service -Name $ServiceName

Write-Host "SudoPulse Connector installed and started successfully!"
