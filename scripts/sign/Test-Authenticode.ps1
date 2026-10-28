#requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$File,
    [string]$ExpectedThumbprint,
    [switch]$RequireTimestamp
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$path = [IO.Path]::GetFullPath($File)
if (!(Test-Path -LiteralPath $path -PathType Leaf)) { throw 'E-SIGN-002: arquivo inexistente' }
$s = Get-AuthenticodeSignature -LiteralPath $path
if ($s.Status -ne 'Valid' -or !$s.SignerCertificate) {
    throw ('E-SIGN-005: Authenticode inválido: ' + $s.Status)
}
if ($ExpectedThumbprint -and
    $s.SignerCertificate.Thumbprint -ine ($ExpectedThumbprint -replace '\s','')) {
    throw 'E-SIGN-005: identidade do editor diferente'
}
if ($RequireTimestamp -and !$s.TimeStamperCertificate) {
    throw 'E-SIGN-005: sem carimbo RFC3161 válido'
}
Write-Host ('PASS Authenticode: ' + $s.SignerCertificate.Subject)
Write-Host ('SHA256: ' + (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant())
