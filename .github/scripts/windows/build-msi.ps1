param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [ValidateSet("amd64", "arm64")]
    [string]$Arch,

    [Parameter(Mandatory = $false)]
    [string]$BinaryPath = "",

    [Parameter(Mandatory = $false)]
    [string]$OutputDir = "packaging"
)

$ErrorActionPreference = "Stop"

$PackageName = "cordium"
$DisplayName = "Cordium"
$Manufacturer = "Octelium Labs, LLC"
$Description = "Cordium - open-source sandbox platform with identity-based, secretless infrastructure access"

$WixPlatform = switch ($Arch) {
    "amd64" { "x64" }
    "arm64" { "ARM64" }
}

$UpgradeCode = "8D67D17B-6F9E-4B8C-9B0F-6E4B63B6E8B1"
$ComponentGuid = "1E3D7469-35D6-4C7E-9D2C-4C0305165791"
$PathGuid = "84286B9D-79F2-4E6F-A6C1-20D28E61F5B3"

if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
    $candidates = @(
        "bin\cordium.exe",
        "dist\cordium-windows-$Arch\cordium.exe",
        "cordium.exe"
    )

    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            $BinaryPath = $candidate
            break
        }
    }
}

if ([string]::IsNullOrWhiteSpace($BinaryPath) -or -not (Test-Path $BinaryPath)) {
    Write-Host "Could not find cordium.exe."
    Write-Host "Expected one of:"
    Write-Host "  bin\cordium.exe"
    Write-Host "  dist\cordium-windows-$Arch\cordium.exe"
    Write-Host "  cordium.exe"
    Write-Host ""
    Write-Host "Existing cordium-like files:"
    Get-ChildItem -Recurse -File -Filter "cordium*" -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty FullName

    throw "Binary file not found"
}

$templatePath = ".github\scripts\windows\template.wxs"

if (-not (Test-Path $templatePath)) {
    throw "Template file not found: $templatePath"
}

New-Item -ItemType Directory -Force -Path "packaging\msi" | Out-Null
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

$resolvedBinaryPath = (Resolve-Path $BinaryPath).Path

Write-Host "Building MSI for Cordium"
Write-Host "Version: $Version"
Write-Host "Arch: $Arch"
Write-Host "WiX Platform: $WixPlatform"
Write-Host "Binary: $resolvedBinaryPath"

$wxsContent = Get-Content $templatePath -Raw
$wxsContent = $wxsContent -replace '\$\{VERSION\}', $Version
$wxsContent = $wxsContent -replace '\$\{UPGRADE_CODE\}', $UpgradeCode
$wxsContent = $wxsContent -replace '\$\{DESCRIPTION\}', $Description
$wxsContent = $wxsContent -replace '\$\{COMPONENT_GUID\}', $ComponentGuid
$wxsContent = $wxsContent -replace '\$\{PATH_GUID\}', $PathGuid
$wxsContent = $wxsContent -replace '\$\{BINARY_PATH\}', ($resolvedBinaryPath -replace '\\', '\\')

$wxsPath = "packaging\msi\cordium.wxs"
$wxsContent | Out-File -FilePath $wxsPath -Encoding UTF8

Write-Host "Generated WXS file: $wxsPath"

$msiPath = Join-Path $OutputDir "cordium-$Version-$Arch.msi"

Write-Host "Building MSI: $msiPath"

wix build `
    -arch $WixPlatform `
    -o $msiPath `
    $wxsPath

if ($LASTEXITCODE -ne 0) {
    throw "WiX build failed with exit code $LASTEXITCODE"
}

Write-Host "Successfully created: $msiPath"