param(
    [Parameter(Mandatory=$true)][ValidatePattern('^[0-9a-f]{64}$')][string]$ExpectedManifestSha256,
    [Parameter(Mandatory=$true)][ValidatePattern('^[0-9a-f]{64}$')][string]$ExpectedHelperSha256,
    [string]$Vault=(Join-Path $env:LOCALAPPDATA 'Rovunet\PanelProductionSigning-v1')
)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
if($PSVersionTable.PSVersion.Major -lt 7) {throw 'Run this command in PowerShell 7.'}
function Assert-ExactFile([string]$Path, [string]$Expected) {
    $taskItem=Get-Item -LiteralPath $Path
    if($taskItem.PSIsContainer -or ($taskItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $taskItem.Length -le 0) {
        throw 'Signing-package inputs must be nonempty regular files.'
    }
    if($Expected -notmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() -cne $Expected) {
        throw 'Signing-package bytes differ from the reviewed hash.'
    }
}
$taskManifest=Join-Path $PSScriptRoot 'RELEASE-MANIFEST.json'
$taskSignature=Join-Path $PSScriptRoot 'RELEASE-MANIFEST.sig'
$taskTool=Join-Path $PSScriptRoot 'signing-vault.exe'
$taskWrapper=Join-Path $PSScriptRoot 'panel-signing-vault.ps1'
$taskInfo=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'CANDIDATE.json') -Raw | ConvertFrom-Json
Assert-ExactFile $taskManifest $ExpectedManifestSha256
Assert-ExactFile $taskTool $ExpectedHelperSha256
Assert-ExactFile $taskWrapper $taskInfo.signing_files.'panel-signing-vault.ps1'.sha256
if($taskInfo.manifest_sha256 -cne $ExpectedManifestSha256 -or
   $taskInfo.signing_files.'signing-vault.exe'.sha256 -cne $ExpectedHelperSha256) {
    throw 'The helper and manifest must belong to the same reviewed signing package.'
}
$taskRelease=Get-Content -LiteralPath $taskManifest -Raw | ConvertFrom-Json
$taskKey='885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe'
if($taskInfo.tag -cne 'v2.1.0' -or $taskInfo.source_commit -cnotmatch '^[0-9a-f]{40}$' -or
   $taskRelease.source_commit -cne $taskInfo.source_commit -or $taskRelease.tag -cne 'v2.1.0' -or
   $taskRelease.repository -cne 'msVPNco/rovunet-panel' -or $taskRelease.environment -cne 'production' -or
   $taskRelease.key_id -cne $taskKey) {
    throw 'The manifest must match the reviewed source, tag and original production identity.'
}
if(Test-Path -LiteralPath $taskSignature) {throw 'A signature already exists; no file will be overwritten.'}
Write-Host ('Checking the existing production vault for source ' + $taskInfo.source_commit)
# The existing sign action checks the expected public identity, private ACLs,
# DPAPI identity and manifest before signing. Its separate check action also
# requires a recovery kit, which this signing-only handoff does not access.
& $taskWrapper -Action sign -Tool $taskTool -Vault $Vault -ExpectedKeyId $taskKey `
    -Manifest $taskManifest -Signature $taskSignature -Tag 'v2.1.0'
if((Get-Item -LiteralPath $taskSignature).Length -ne 64) {throw 'The vault did not produce a valid-length signature.'}
Assert-ExactFile $taskManifest $ExpectedManifestSha256
Write-Host 'Done. Return RELEASE-MANIFEST.json and RELEASE-MANIFEST.sig from this folder.'
