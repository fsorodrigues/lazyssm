#!/usr/bin/env pwsh

param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$Message,

    [Parameter(Position = 1)]
    [string]$IntervalSeconds = "1",

    [Parameter(Position = 2)]
    [string]$MaxRuntimeSeconds = $env:SIMULATE_SERVICE_MAX_RUNTIME_SECONDS
)

if ([string]::IsNullOrWhiteSpace($MaxRuntimeSeconds)) {
    $MaxRuntimeSeconds = "0"
}

$parsedInterval = 0
if (-not [int]::TryParse($IntervalSeconds, [ref]$parsedInterval) -or $parsedInterval -lt 1) {
    [Console]::Error.WriteLine("interval_seconds must be a positive integer")
    exit 1
}

$parsedMaxRuntime = 0
if (-not [int]::TryParse($MaxRuntimeSeconds, [ref]$parsedMaxRuntime) -or $parsedMaxRuntime -lt 0) {
    [Console]::Error.WriteLine("max_runtime_seconds must be a non-negative integer")
    exit 1
}

$interval = $parsedInterval
$maxRuntime = $parsedMaxRuntime
$start = Get-Date

while ($true) {
    $elapsed = [int]((Get-Date) - $start).TotalSeconds

    if ($maxRuntime -gt 0 -and $elapsed -ge $maxRuntime) {
        $timestamp = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        Write-Output "$timestamp $Message timed out after ${maxRuntime}s"
        exit 0
    }

    $timestamp = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    Write-Output "$timestamp $Message"
    Start-Sleep -Seconds $interval
}
