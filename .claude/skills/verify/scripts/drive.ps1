# Drive the nmf Windows build: send keys and/or capture a window screenshot.
# FindWindow-by-title fails from WSL interop; resolve via MainWindowHandle.
#
# Usage (from WSL; copy this file to a Windows path first, e.g. C:\Temp):
#   powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\\Temp\\drive.ps1" `
#       -Proc nmf -Keys "{DOWN}{ENTER}" -Shot "C:\\Temp\\out.png"
# SendKeys syntax: {DOWN} {UP} {ENTER} {BACKSPACE} {F2}; "+." = Shift+Period; "q" = letter.
param(
    [string]$Proc = "nmf",
    [string]$Keys = "",
    [string]$Shot = ""
)
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class Win32d {
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
    public struct RECT { public int Left, Top, Right, Bottom; }
}
"@
$p = Get-Process $Proc -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
if (-not $p) { Write-Output "PROCESS_NOT_FOUND $Proc"; exit 1 }
if ($Keys -ne "") {
    $sh = New-Object -ComObject WScript.Shell
    $sh.AppActivate($p.Id) | Out-Null
    Start-Sleep -Milliseconds 400
    [System.Windows.Forms.SendKeys]::SendWait($Keys)
    Write-Output "SENT $Keys"
}
if ($Shot -ne "") {
    $r = New-Object Win32d+RECT
    [Win32d]::GetWindowRect($p.MainWindowHandle, [ref]$r) | Out-Null
    $w = $r.Right - $r.Left; $ht = $r.Bottom - $r.Top
    if ($w -le 0 -or $ht -le 0) { Write-Output "BAD_RECT"; exit 1 }
    $bmp = New-Object System.Drawing.Bitmap $w, $ht
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($r.Left, $r.Top, 0, 0, (New-Object System.Drawing.Size($w, $ht)))
    $bmp.Save($Shot, [System.Drawing.Imaging.ImageFormat]::Png)
    Write-Output "SAVED $Shot ${w}x${ht}"
}
