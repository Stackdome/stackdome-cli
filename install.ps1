& {
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$previousProgressPreference = $ProgressPreference
$ProgressPreference = 'SilentlyContinue'

function Get-ConfiguredValue {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,

        [Parameter(Mandatory = $true)]
        [string]$DefaultValue
    )

    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $DefaultValue
    }
    return $value.Trim()
}

function Get-StackdomeArchitecture {
    $architectureOverride = [Environment]::GetEnvironmentVariable('STACKDOME_ARCH')
    if (-not [string]::IsNullOrWhiteSpace($architectureOverride)) {
        switch ($architectureOverride.Trim().ToLowerInvariant()) {
            'amd64' { return 'amd64' }
            'x64' { return 'amd64' }
            'arm64' { return 'arm64' }
            default {
                throw "Unsupported Windows architecture override '$architectureOverride'. Set STACKDOME_ARCH to AMD64 or ARM64."
            }
        }
    }

    # PROCESSOR_ARCHITEW6432 reports the native machine architecture when a
    # 32-bit or x64 process is emulated on ARM64 Windows.
    $detectedArchitecture = [Environment]::GetEnvironmentVariable('PROCESSOR_ARCHITEW6432')
    if ([string]::IsNullOrWhiteSpace($detectedArchitecture)) {
        $detectedArchitecture = [Environment]::GetEnvironmentVariable('PROCESSOR_ARCHITECTURE')
    }
    if ([string]::IsNullOrWhiteSpace($detectedArchitecture)) {
        try {
            $detectedArchitecture = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
        }
        catch {
            throw 'Unable to detect the Windows architecture. Set STACKDOME_ARCH to AMD64 or ARM64.'
        }
    }

    switch ($detectedArchitecture.Trim().ToUpperInvariant()) {
        'AMD64' { 'amd64' }
        'X64' { 'amd64' }
        'ARM64' { 'arm64' }
        default {
            throw "Unsupported Windows architecture '$detectedArchitecture'. Stackdome provides installers for AMD64 and ARM64."
        }
    }
}

function Get-LatestStackdomeVersion {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ApiBaseUrl,

        [Parameter(Mandatory = $true)]
        [string]$Repository
    )

    $latestReleaseUrl = '{0}/repos/{1}/releases/latest' -f $ApiBaseUrl.TrimEnd('/'), $Repository
    try {
        $release = Invoke-RestMethod -Uri $latestReleaseUrl -UseBasicParsing -Headers @{
            Accept = 'application/vnd.github+json'
            'User-Agent' = 'stackdome-installer'
        }
    }
    catch {
        throw "Unable to discover the latest Stackdome release for repository '$Repository'. Check network access or set STACKDOME_VERSION explicitly."
    }

    $tagNameProperty = $release.PSObject.Properties['tag_name']
    if ($null -eq $tagNameProperty) {
        throw "The latest-release response for repository '$Repository' did not contain a tag_name. Set STACKDOME_VERSION explicitly."
    }
    $version = [string]$tagNameProperty.Value
    if ([string]::IsNullOrWhiteSpace($version)) {
        throw "The latest-release response for repository '$Repository' contained an empty tag_name. Set STACKDOME_VERSION explicitly."
    }
    return $version.Trim()
}

function Save-StackdomeFile {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Uri,

        [Parameter(Mandatory = $true)]
        [string]$Destination,

        [Parameter(Mandatory = $true)]
        [string]$Description
    )

    try {
        Invoke-WebRequest -Uri $Uri -OutFile $Destination -UseBasicParsing
    }
    catch {
        throw "Unable to download $Description. Verify that the release and asset exist, then retry."
    }
}

function Test-PathContains {
    param(
        [AllowNull()]
        [string]$PathValue,

        [Parameter(Mandatory = $true)]
        [string]$Entry
    )

    if ([string]::IsNullOrWhiteSpace($PathValue)) {
        return $false
    }

    $normalizedEntry = $Entry.Trim().Trim('"').TrimEnd([char[]]'\/')
    foreach ($candidate in ($PathValue -split [IO.Path]::PathSeparator)) {
        $normalizedCandidate = $candidate.Trim().Trim('"').TrimEnd([char[]]'\/')
        if ([StringComparer]::OrdinalIgnoreCase.Equals($normalizedCandidate, $normalizedEntry)) {
            return $true
        }
    }
    return $false
}

$repository = Get-ConfiguredValue -Name 'STACKDOME_REPOSITORY' -DefaultValue 'Stackdome/stackdome-cli'
if ($repository -notmatch '^[^/\s]+/[^/\s]+$') {
    throw "STACKDOME_REPOSITORY must use the owner/repository format; received '$repository'."
}

$version = [Environment]::GetEnvironmentVariable('STACKDOME_VERSION')
if ([string]::IsNullOrWhiteSpace($version)) {
    $apiBaseUrl = Get-ConfiguredValue -Name 'STACKDOME_API_BASE_URL' -DefaultValue 'https://api.github.com'
    $version = Get-LatestStackdomeVersion -ApiBaseUrl $apiBaseUrl -Repository $repository
}
else {
    $version = $version.Trim()
}
if ($version -notmatch '^[A-Za-z0-9._-]+$') {
    throw "Invalid release version '$version'. Use a tag containing only letters, numbers, dots, underscores, or hyphens."
}

$architecture = Get-StackdomeArchitecture
$assetName = 'stackdome_{0}_windows_{1}.zip' -f $version, $architecture
$releaseBaseUrl = Get-ConfiguredValue -Name 'STACKDOME_RELEASE_BASE_URL' -DefaultValue "https://github.com/$repository/releases/download"
$releaseBaseUrl = $releaseBaseUrl.TrimEnd('/')
$assetUrl = '{0}/{1}/{2}' -f $releaseBaseUrl, $version, $assetName
$checksumsUrl = '{0}/{1}/checksums.txt' -f $releaseBaseUrl, $version

$configuredInstallDir = [Environment]::GetEnvironmentVariable('STACKDOME_INSTALL_DIR')
if ([string]::IsNullOrWhiteSpace($configuredInstallDir)) {
    $localAppData = [Environment]::GetFolderPath('LocalApplicationData')
    if ([string]::IsNullOrWhiteSpace($localAppData)) {
        $localAppData = $env:LOCALAPPDATA
    }
    if ([string]::IsNullOrWhiteSpace($localAppData)) {
        throw 'Unable to determine a user-writable install directory. Set STACKDOME_INSTALL_DIR explicitly.'
    }
    $installDir = Join-Path $localAppData 'Programs\Stackdome\bin'
}
else {
    $installDir = [Environment]::ExpandEnvironmentVariables($configuredInstallDir.Trim())
}

$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('stackdome-install-{0}' -f [Guid]::NewGuid().ToString('N'))
$archivePath = Join-Path $tempRoot $assetName
$checksumsPath = Join-Path $tempRoot 'checksums.txt'
$extractPath = Join-Path $tempRoot 'extracted'

try {
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
    Save-StackdomeFile -Uri $assetUrl -Destination $archivePath -Description $assetName
    Save-StackdomeFile -Uri $checksumsUrl -Destination $checksumsPath -Description 'checksums.txt'

    $checksumManifest = Get-Content -LiteralPath $checksumsPath -Raw
    $checksumPattern = '(?im)^(?<hash>[0-9a-f]{64})[ \t]+\*?' + [regex]::Escape($assetName) + '[ \t]*$'
    $checksumMatch = [regex]::Match($checksumManifest, $checksumPattern)
    if (-not $checksumMatch.Success) {
        throw "checksums.txt does not contain an exact SHA-256 entry for $assetName. The release may be incomplete."
    }

    $expectedHash = $checksumMatch.Groups['hash'].Value.ToLowerInvariant()
    $actualHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) {
        throw "Checksum mismatch for $assetName. Expected $expectedHash but downloaded $actualHash. Delete any cached copy and retry."
    }

    try {
        Expand-Archive -LiteralPath $archivePath -DestinationPath $extractPath -Force
    }
    catch {
        throw "Unable to extract the verified archive $assetName. Check available disk space and retry."
    }

    $executablePath = Join-Path $extractPath 'stackdome.exe'
    if (-not (Test-Path -LiteralPath $executablePath -PathType Leaf)) {
        throw "The verified archive $assetName does not contain stackdome.exe."
    }

    try {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        Copy-Item -LiteralPath $executablePath -Destination (Join-Path $installDir 'stackdome.exe') -Force
    }
    catch {
        throw "Unable to install stackdome.exe in '$installDir'. Set STACKDOME_INSTALL_DIR to a writable directory and retry."
    }

    $skipPathUpdate = [Environment]::GetEnvironmentVariable('STACKDOME_SKIP_PATH_UPDATE')
    if ($skipPathUpdate -ne '1') {
        if (-not (Test-PathContains -PathValue $env:Path -Entry $installDir)) {
            if ([string]::IsNullOrWhiteSpace($env:Path)) {
                $env:Path = $installDir
            }
            else {
                $env:Path = $installDir + [IO.Path]::PathSeparator + $env:Path
            }
        }

        try {
            $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
            if (-not (Test-PathContains -PathValue $userPath -Entry $installDir)) {
                if ([string]::IsNullOrWhiteSpace($userPath)) {
                    $updatedUserPath = $installDir
                }
                else {
                    $updatedUserPath = $userPath + [IO.Path]::PathSeparator + $installDir
                }
                [Environment]::SetEnvironmentVariable('Path', $updatedUserPath, 'User')
            }
        }
        catch {
            Write-Warning "Stackdome was installed, but the user PATH could not be updated. Add '$installDir' to your user PATH manually."
        }
    }

    Write-Host "Stackdome CLI $version was installed to $installDir\stackdome.exe"
    Write-Host "Restart your terminal, then run 'stackdome version' to verify the installation."
}
finally {
    $ProgressPreference = $previousProgressPreference
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
}
