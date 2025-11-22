# Builds Windows and Linux binaries with size optimizations into ./dist.
$ErrorActionPreference = "Stop"

$repoRoot = $PSScriptRoot
Set-Location $repoRoot

$dist = Join-Path $repoRoot "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null

$targets = @(
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe" },
    @{ OS = "linux";   Arch = "amd64"; Ext = "" }
)

$originalEnv = @{
    GOOS        = $env:GOOS
    GOARCH      = $env:GOARCH
    CGO_ENABLED = $env:CGO_ENABLED
}

try {
    foreach ($target in $targets) {
        $env:GOOS = $target.OS
        $env:GOARCH = $target.Arch
        $env:CGO_ENABLED = "0"

        $name = "TwitchChannelPointsMiner-$($target.OS)-$($target.Arch)$($target.Ext)"
        $outputPath = Join-Path $dist $name

        Write-Host "Building $name..."
        go build -trimpath -buildvcs=false -ldflags "-s -w" -o $outputPath .
    }
}
finally {
    $env:GOOS = $originalEnv.GOOS
    $env:GOARCH = $originalEnv.GOARCH
    $env:CGO_ENABLED = $originalEnv.CGO_ENABLED
}

Write-Host "Builds complete -> $dist"
