#requires -Version 5.1
# Sign an EXE after SFX assembly. Never overwrite the source or an existing output.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$InputFile,
    [Parameter(Mandatory=$true)][string]$OutputFile,
    [Parameter(Mandatory=$true)][string]$Thumbprint,
    [string]$TimestampUrl,
    [string]$SignToolPath,
    [string]$ExpectedSHA256,
    [switch]$MachineStore,
    [switch]$LabNoTimestamp
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
function Find-SignTool {
    param([string]$Requested)
    if ($Requested) {
        $tool = [IO.Path]::GetFullPath($Requested)
        if (!(Test-Path -LiteralPath $tool -PathType Leaf)) { throw 'E-SIGN-001: signtool não encontrado' }
        return $tool
    }
    $command = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    $found = @()
    foreach ($base in @([Environment]::GetFolderPath('ProgramFilesX86'),
                        [Environment]::GetFolderPath('ProgramFiles'))) {
        if (!$base) { continue }
        $glob = Join-Path $base 'Windows Kits\10\bin\*\x64\signtool.exe'
        $found += @(Get-ChildItem -Path $glob -File -ErrorAction SilentlyContinue)
    }
    if (!$found.Count) { throw 'E-SIGN-001: SignTool ausente; instale o Windows SDK' }
    return ($found | Sort-Object FullName -Descending | Select-Object -First 1).FullName
}
$source = [IO.Path]::GetFullPath($InputFile)
$target = [IO.Path]::GetFullPath($OutputFile)
if ($source -ieq $target -or [IO.Path]::GetExtension($source) -ine '.exe' -or
    [IO.Path]::GetExtension($target) -ine '.exe') {
    throw 'E-SIGN-002: origem/destino inválidos ou não são EXE'
}
if (Test-Path -LiteralPath $target) { throw 'E-SIGN-002: destino já existe' }
$item = Get-Item -LiteralPath $source -ErrorAction Stop
if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw 'E-SIGN-002: origem não é arquivo EXE regular'
}
$f = [IO.File]::OpenRead($source)
try {
    $header = New-Object byte[] 2
    if ($f.Read($header, 0, 2) -ne 2 -or $header[0] -ne 77 -or $header[1] -ne 90) {
        throw 'E-SIGN-002: arquivo sem cabeçalho MZ'
    }
} finally { $f.Dispose() }
if ($ExpectedSHA256) {
    if ($ExpectedSHA256 -notmatch '^[0-9a-fA-F]{64}$' -or
        (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash -ine $ExpectedSHA256) {
        throw 'E-SIGN-002: hash SHA256 da origem não confere'
    }
}
$thumb = ($Thumbprint -replace '\s', '').ToUpperInvariant()
if ($thumb -notmatch '^[A-F0-9]{40}$') { throw 'E-SIGN-003: thumbprint inválido' }
$scope = if ($MachineStore) { 'LocalMachine' } else { 'CurrentUser' }
$cert = Get-Item -LiteralPath ("Cert:\" + $scope + "\My\" + $thumb) -ErrorAction Stop
if (!$cert.HasPrivateKey -or $cert.NotBefore -gt (Get-Date) -or $cert.NotAfter -le (Get-Date)) {
    throw 'E-SIGN-003: certificado sem chave privada ou fora da validade'
}
$validEKU = $false
foreach ($extension in $cert.Extensions) {
    if ($extension -is [System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension]) {
        foreach ($oid in $extension.EnhancedKeyUsages) {
            if ($oid.Value -eq '1.3.6.1.5.5.7.3.3') { $validEKU = $true }
        }
    }
}
if (!$validEKU) { throw 'E-SIGN-003: EKU Code Signing ausente' }
if ($LabNoTimestamp) {
    if ($TimestampUrl -or $cert.Subject -notmatch 'CN=DeployCraft CI Test(,|$)') {
        throw 'E-SIGN-004: sem timestamp permitido apenas para certificado CI de teste'
    }
} elseif (!$TimestampUrl -or $TimestampUrl -notmatch '^https?://') {
    throw 'E-SIGN-004: informe um servidor RFC3161 em -TimestampUrl'
}
$tool = Find-SignTool -Requested $SignToolPath
$directory = Split-Path -Parent $target
if (!(Test-Path -LiteralPath $directory -PathType Container)) {
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
}
$tmp = Join-Path $directory ('.DeployCraft-Sign-' + [Guid]::NewGuid().ToString('N') + '.exe')
try {
    Copy-Item -LiteralPath $source -Destination $tmp -ErrorAction Stop
    $argsSign = @('sign','/s','My','/sha1',$thumb,'/fd','SHA256')
    if ($MachineStore) { $argsSign += '/sm' }
    if (!$LabNoTimestamp) { $argsSign += @('/tr',$TimestampUrl,'/td','SHA256') }
    $argsSign += $tmp
    & $tool @argsSign
    if ($LASTEXITCODE -ne 0) { throw 'E-SIGN-005: falha no signtool sign' }
    & $tool verify /pa $tmp
    if ($LASTEXITCODE -ne 0) { throw 'E-SIGN-005: signtool verify /pa falhou' }
    $actual = Get-AuthenticodeSignature -LiteralPath $tmp
    if ($actual.Status -ne 'Valid' -or !$actual.SignerCertificate -or
        $actual.SignerCertificate.Thumbprint -ine $thumb) {
        throw ("E-SIGN-005: assinatura ou editor inválido: " + $actual.Status)
    }
    if (!$LabNoTimestamp -and !$actual.TimeStamperCertificate) {
        throw 'E-SIGN-005: certificado de timestamp RFC3161 ausente'
    }
    if (Test-Path -LiteralPath $target) { throw 'E-SIGN-002: destino criado em paralelo' }
    Move-Item -LiteralPath $tmp -Destination $target -ErrorAction Stop
    Write-Host ("PASS Authenticode: " + $actual.SignerCertificate.Subject)
    Write-Host ("SHA256 final: " + (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant())
    if ($LabNoTimestamp) { Write-Warning 'CERTIFICADO DE TESTE, não usar em produção' }
} finally {
    if (Test-Path -LiteralPath $tmp) {
        Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
    }
}
