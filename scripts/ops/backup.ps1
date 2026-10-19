# F6-7 (Windows dev): backup database uji/dev memakai mysqldump Laragon.
# Produksi memakai scripts/ops/backup.sh di Linux. Jangan pakai skrip ini ke produksi.
param(
  [string]$DbName = "mini_erp_db",
  [string]$BackupDir = "./backups"
)
$ErrorActionPreference = "Stop"
$mysqldump = "C:/laragon/bin/mysql/mysql-8.4.3-winx64/bin/mysqldump.exe"
if (-not (Test-Path -LiteralPath $mysqldump)) { throw "mysqldump tak ditemukan: $mysqldump" }
New-Item -ItemType Directory -Path $BackupDir -Force | Out-Null
$ts = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
$out = Join-Path $BackupDir "db-${ts}.sql.gz"
Write-Host "backup: database $DbName -> $out"
# Catatan: password dibaca dari MYSQL_PWD env bila diset; tanpa itu root tanpa password (default Laragon).
& $mysqldump -h 127.0.0.1 -u root $DbName --single-transaction --routines --triggers --events --hex-blob --default-character-set=utf8mb4 --set-gtid-purged=OFF --no-tablespaces |
  & { param($i) $bytes = [Text.Encoding]::UTF8.GetBytes($i -join "`n"); $ms = New-Object IO.MemoryStream; $gz = New-Object IO.Compression.GzipStream($ms, [IO.Compression.CompressionMode]::Compress); $gz.Write($bytes, 0, $bytes.Length); $gz.Close(); [IO.File]::WriteAllBytes($out, $ms.ToArray()) }
Write-Host "backup: selesai $out"
