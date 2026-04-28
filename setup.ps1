# CloudOS Windows Installer Script
# This script sets up CloudOS as a native Windows application.

$AppName = "CloudOS"
$InstallDir = Join-Path $env:LOCALAPPDATA "CloudOS"
$BinaryName = "cloudos_app.exe"
$SourceBinary = Join-Path $env:USERPROFILE $BinaryName

Write-Host "🚀 Starting CloudOS Installation..." -ForegroundColor Cyan

# 1. Create Installation Directory
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
    Write-Host "✅ Created installation directory: $InstallDir"
}

# 2. Copy Binary
if (Test-Path $SourceBinary) {
    Copy-Item $SourceBinary (Join-Path $InstallDir $BinaryName) -Force
    Write-Host "✅ Copied application binary."
} else {
    Write-Host "❌ Error: Could not find built binary at $SourceBinary" -ForegroundColor Red
    exit
}

# 3. Handle Icon (Convert PNG to ICO placeholder or just use PNG)
# Since we can't easily convert PNG to ICO in PS without libraries, we'll use a standard icon
# or just reference the PNG if the shortcut allows it.
$SourceIcon = "C:\Users\AYUSH.G.PAL\.gemini\antigravity\brain\98c886c0-ae44-45fd-9584-8d34c56e7be2\cloudos_app_icon_1777265452072.png"
if (Test-Path $SourceIcon) {
    Copy-Item $SourceIcon (Join-Path $InstallDir "app_icon.png") -Force
    Write-Host "✅ Copied application icon."
}

# 4. Create Desktop Shortcut
$WshShell = New-Object -ComObject WScript.Shell
$ShortcutPath = Join-Path ([Environment]::GetFolderPath("Desktop")) "$AppName.lnk"
$Shortcut = $WshShell.CreateShortcut($ShortcutPath)
$Shortcut.TargetPath = Join-Path $InstallDir $BinaryName
$Shortcut.Arguments = "gui"
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "Local-First Personal Cloud OS"
$Shortcut.IconLocation = "$InstallDir\$BinaryName,0" # Default icon for now
$Shortcut.Save()
Write-Host "✅ Created Desktop Shortcut."

# 5. Create Start Menu Shortcut
$StartMenuPath = Join-Path ([Environment]::GetFolderPath("Programs")) "$AppName.lnk"
$Shortcut = $WshShell.CreateShortcut($StartMenuPath)
$Shortcut.TargetPath = Join-Path $InstallDir $BinaryName
$Shortcut.Arguments = "gui"
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Save()
Write-Host "✅ Added to Start Menu."

Write-Host "`n✨ CloudOS has been successfully installed!" -ForegroundColor Green
Write-Host "You can now launch it from your Desktop or Start Menu."
Write-Host "----------------------------------------------------"
