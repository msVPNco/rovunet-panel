param(
    [Parameter(Mandatory=$true)][ValidateSet('init','check','recover','sign')][string]$Action,
    [Parameter(Mandatory=$true)][string]$Tool,
    [string]$Vault=(Join-Path $env:LOCALAPPDATA 'Rovunet\PanelProductionSigning-v1'),
    [string]$Recovery,
    [string]$Manifest,
    [string]$Signature,
    [string]$Tag,
    [string]$ExpectedKeyId='885d8aba22365ce42e20527bfa00e82aed3cba2f69501f146ed0496a8bc3dcbe',
    [switch]$PortableRecovery
)
$ErrorActionPreference='Stop'
$taskUser=[System.Security.Principal.WindowsIdentity]::GetCurrent().User
$taskSystem=[System.Security.Principal.SecurityIdentifier]::new('S-1-5-18')
function Assert-PrivateFolder([string]$Path) {
    $taskInfo=Get-Item -LiteralPath $Path
    if(-not $taskInfo.PSIsContainer -or ($taskInfo.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw 'Vault/recovery must be a regular directory, not a link.'
    }
    $taskAcl=Get-Acl -LiteralPath $Path
    if(-not $taskAcl.AreAccessRulesProtected -or $taskAcl.GetOwner([System.Security.Principal.SecurityIdentifier]).Value -ne $taskUser.Value) {
        throw 'Vault/recovery ACL or owner is not private to the current Windows account.'
    }
    foreach($taskRule in $taskAcl.GetAccessRules($true,$true,[System.Security.Principal.SecurityIdentifier])) {
        if($taskRule.AccessControlType -eq 'Allow' -and $taskRule.IdentityReference.Value -notin @($taskUser.Value,$taskSystem.Value)) {
            throw 'Vault/recovery grants access to another Windows principal.'
        }
    }
}
function Assert-PrivateFile([string]$Path) {
    $taskInfo=Get-Item -LiteralPath $Path
    if($taskInfo.PSIsContainer -or ($taskInfo.Attributes -band [IO.FileAttributes]::ReparsePoint)) {throw 'Vault input must be a regular file.'}
    $taskAcl=Get-Acl -LiteralPath $Path
    if($taskAcl.GetOwner([System.Security.Principal.SecurityIdentifier]).Value -ne $taskUser.Value) {throw 'Vault input has a different owner.'}
    foreach($taskRule in $taskAcl.GetAccessRules($true,$true,[System.Security.Principal.SecurityIdentifier])) {
        if($taskRule.AccessControlType -eq 'Allow' -and $taskRule.IdentityReference.Value -notin @($taskUser.Value,$taskSystem.Value)) {
            throw 'Vault input grants access to another Windows principal.'
        }
    }
}
function New-PrivateFolder([string]$Path) {
    $taskResolved=[IO.Path]::GetFullPath($Path)
    if(Test-Path -LiteralPath $taskResolved) {
        # Never rewrite ownership/ACLs on an existing operator directory.
        Assert-PrivateFolder $taskResolved
        return
    }
    [void][IO.Directory]::CreateDirectory($taskResolved)
    $taskInfo=Get-Item -LiteralPath $taskResolved
    if(-not $taskInfo.PSIsContainer -or ($taskInfo.Attributes -band [IO.FileAttributes]::ReparsePoint)) {throw 'Private directory cannot be a link.'}
    $taskAcl=[System.Security.AccessControl.DirectorySecurity]::new()
    $taskAcl.SetAccessRuleProtection($true,$false)
    $taskAcl.SetOwner($taskUser)
    foreach($taskPrincipal in @($taskUser,$taskSystem)) {
        $taskAcl.AddAccessRule([System.Security.AccessControl.FileSystemAccessRule]::new(
            $taskPrincipal,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
    }
    Set-Acl -LiteralPath $taskResolved -AclObject $taskAcl
    Assert-PrivateFolder $taskResolved
}
$Vault=[IO.Path]::GetFullPath($Vault)
if($PortableRecovery -and $Action -notin @('recover','check')) {throw 'Portable input is only supported for recovery/check; destination vault stays private.'}
if($Action -eq 'init' -or $Action -eq 'recover') {New-PrivateFolder $Vault} else {Assert-PrivateFolder $Vault}
if($Action -eq 'check' -or $Action -eq 'sign') {
    foreach($taskName in @('production.dpapi','identity-public.json')) {Assert-PrivateFile (Join-Path $Vault $taskName)}
}
$taskArgs=@($Action,'--vault',$Vault)
if($Action -ne 'init') {$taskArgs+=@('--expected-key-id',$ExpectedKeyId)}
if($Recovery) {
    $Recovery=[IO.Path]::GetFullPath($Recovery)
    if($Action -eq 'init') {New-PrivateFolder $Recovery}
    elseif($PortableRecovery) {
        # Removable/exFAT media may carry no current-account NTFS owner.
        # It remains untrusted read-only input, bound to ExpectedKeyId and age
        # authentication in the helper. Never loosen the destination vault.
        $taskInfo=Get-Item -LiteralPath $Recovery
        if(-not $taskInfo.PSIsContainer -or ($taskInfo.Attributes -band [IO.FileAttributes]::ReparsePoint)) {throw 'Portable recovery input cannot be a link.'}
    } else {Assert-PrivateFolder $Recovery}
    if($Action -ne 'init' -and -not $PortableRecovery) {
        foreach($taskName in @('recovery.agekey','production-backup.age','identity-public.json')) {Assert-PrivateFile (Join-Path $Recovery $taskName)}
    }
    $taskArgs+=@('--recovery',$Recovery)
}
if($Action -eq 'sign') {
    if(-not $Manifest -or -not $Signature -or -not $Tag){throw 'Manifest, Signature and Tag are required.'}
    $taskArgs+=@('--manifest',[IO.Path]::GetFullPath($Manifest),'--signature',[IO.Path]::GetFullPath($Signature),'--tag',$Tag)
}
& $Tool @taskArgs
if($LASTEXITCODE -ne 0){throw 'Signing vault operation failed; private contents were not printed.'}
