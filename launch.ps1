param([switch]$Demo,[string]$Addr,[switch]$Stop,[switch]$NoBrowser)
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$taskRoot=$PSScriptRoot
$taskRuntime=Join-Path $taskRoot '.runtime'
$taskDataDir=Join-Path $taskRoot 'data'
$taskData=Join-Path $taskDataDir $(if($Demo){'demo-records.json'}else{'records.json'})
$taskSession=Join-Path $taskDataDir $(if($Demo){'.session-demo.json'}else{'.session-real.json'})
$taskExe=Join-Path $taskRoot 'ledger.exe'
if(-not(Test-Path -LiteralPath $taskExe)){
 $taskExe=Join-Path $taskRuntime 'ledger.exe'
 if(-not $Stop){
  $taskSources=@(Get-ChildItem -LiteralPath (Join-Path $taskRoot 'cmd'),(Join-Path $taskRoot 'internal'),(Join-Path $taskRoot 'web') -Recurse -File | Where-Object {$_.Extension -in '.go','.js','.css','.html'})
  $taskNewest=($taskSources|Sort-Object LastWriteTimeUtc -Descending|Select-Object -First 1).LastWriteTimeUtc
  if(-not(Test-Path -LiteralPath $taskExe) -or (Get-Item -LiteralPath $taskExe).LastWriteTimeUtc -lt $taskNewest){
   $env:Path=[Environment]::GetEnvironmentVariable('Path','Machine')+';'+[Environment]::GetEnvironmentVariable('Path','User')
   $taskGo=Get-Command go.exe -ErrorAction SilentlyContinue
   if(-not $taskGo){throw 'Go 1.22 or newer is required. Alternatively, use the Windows ready-to-run bundle.'}
   New-Item -ItemType Directory -Force -Path $taskRuntime|Out-Null
   Push-Location -LiteralPath $taskRoot
   try{Write-Host 'Preparing the application for the first launch...'; & $taskGo.Source build -trimpath -o $taskExe ./cmd/ledger;if($LASTEXITCODE -ne 0){throw 'Build failed.'}}finally{Pop-Location}
  }
 }
}
$taskHash=[Security.Cryptography.SHA256]::Create()
try{$taskKey=([BitConverter]::ToString($taskHash.ComputeHash([Text.Encoding]::UTF8.GetBytes($taskData.ToLowerInvariant())))).Replace('-','')}finally{$taskHash.Dispose()}
$taskMutex=[Threading.Mutex]::new($false,('Local\PersonalLedger-'+$taskKey))
$taskOwns=$false
function Get-LedgerSession {
 try{
  if(-not(Test-Path -LiteralPath $taskSession)){return $null}
  $taskSaved=Get-Content -LiteralPath $taskSession -Raw|ConvertFrom-Json
  if($taskSaved.exe -ine $taskExe -or $taskSaved.data -ine $taskData){return $null}
  $taskRunning=Get-Process -Id ([int]$taskSaved.pid) -ErrorAction SilentlyContinue
  if(-not $taskRunning -or $taskRunning.Path -ine $taskExe){return $null}
  $taskUri=[Uri]$taskSaved.url
  if($taskUri.Host -ne '127.0.0.1'){return $null}
  $taskConnection=Get-NetTCPConnection -State Listen -LocalPort $taskUri.Port -ErrorAction SilentlyContinue|Where-Object OwningProcess -eq $taskRunning.Id
  if(-not $taskConnection){return $null}
  $taskMeta=Invoke-RestMethod ($taskSaved.url+'/api/meta') -TimeoutSec 2
  if([bool]$taskMeta.demo -ne [bool]$Demo){return $null}
  return $taskSaved
 }catch{return $null}
}
try{
 try{$taskOwns=$taskMutex.WaitOne(30000)}catch [Threading.AbandonedMutexException]{$taskOwns=$true}
 if(-not $taskOwns){throw 'Another launch is still being prepared. Please try again.'}
 $taskExisting=Get-LedgerSession
 if($Stop){
  if($taskExisting){Stop-Process -Id ([int]$taskExisting.pid);Write-Host 'Ledger stopped.'}else{Write-Host 'No running ledger found.'}
  exit 0
 }
 if($taskExisting){
  if(-not $NoBrowser){Start-Process $taskExisting.url}
  [pscustomobject]@{url=$taskExisting.url;pid=$taskExisting.pid;reused=$true}|ConvertTo-Json -Compress
  exit 0
 }
 if($Addr){
  if($Addr -notmatch '^127\.0\.0\.1:(\d{1,5})$' -or [int]$Matches[1] -lt 1 -or [int]$Matches[1] -gt 65535){throw 'Use -Addr 127.0.0.1:PORT.'}
  $taskPort=[int]$Matches[1]
  if(Get-NetTCPConnection -State Listen -LocalPort $taskPort -ErrorAction SilentlyContinue){throw 'The selected port is already occupied.'}
 }else{
  $taskUsed=@(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue|Select-Object -ExpandProperty LocalPort)
  $taskPort=8080..8199|Where-Object {$_ -notin $taskUsed}|Select-Object -First 1
  if(-not $taskPort){throw 'No free local port was found.'}
 }
 New-Item -ItemType Directory -Force -Path $taskDataDir|Out-Null
 $taskArgs=@('-addr',('127.0.0.1:'+$taskPort),'-data',('"'+$taskData+'"'))
 if($Demo){$taskArgs+='-demo'}
 $taskLog=Join-Path $taskDataDir $(if($Demo){'demo-server.log'}else{'server.log'})
 $taskOut=Join-Path $taskDataDir $(if($Demo){'demo-output.log'}else{'output.log'})
 $taskProcess=Start-Process -FilePath $taskExe -ArgumentList $taskArgs -WorkingDirectory $taskRoot -WindowStyle Hidden -PassThru -RedirectStandardError $taskLog -RedirectStandardOutput $taskOut
 $taskURL='http://127.0.0.1:'+$taskPort
 $taskReady=$false;$taskDeadline=[DateTime]::UtcNow.AddSeconds(20)
 while([DateTime]::UtcNow -lt $taskDeadline){
  $taskProcess.Refresh();if($taskProcess.HasExited){throw 'Ledger exited. See data/server.log or data/demo-server.log.'}
  try{$taskMeta=Invoke-RestMethod ($taskURL+'/api/meta') -TimeoutSec 1;if($taskMeta.version){$taskReady=$true;break}}catch{}
  Start-Sleep -Milliseconds 100
 }
 if(-not $taskReady){Stop-Process -Id $taskProcess.Id -ErrorAction SilentlyContinue;throw 'The application did not become ready.'}
 $taskInfo=[pscustomobject]@{url=$taskURL;pid=$taskProcess.Id;exe=$taskExe;data=$taskData}
 $taskInfo|ConvertTo-Json|Set-Content -LiteralPath $taskSession -Encoding UTF8
 if(-not $NoBrowser){Start-Process $taskURL}
 [pscustomobject]@{url=$taskURL;pid=$taskProcess.Id;reused=$false}|ConvertTo-Json -Compress
}finally{if($taskOwns){$taskMutex.ReleaseMutex()};$taskMutex.Dispose()}
