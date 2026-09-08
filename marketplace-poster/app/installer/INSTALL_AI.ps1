param([string]$InstalledAppDir = '', [switch]$Silent, [switch]$Repair)
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$CpuUrl = 'https://github.com/ggml-org/llama.cpp/releases/download/b10516/llama-b10516-bin-win-cpu-x64.zip'
$CpuSha = 'fbbbc55e0eb2e1b07f9dcb9488616c98ed47d9003b90e15e7c8c7812c4307cd3'
$VulkanUrl = 'https://github.com/ggml-org/llama.cpp/releases/download/b10516/llama-b10516-bin-win-vulkan-x64.zip'
$VulkanSha = '530f57d2a874ce017827c1e5a926812b9d5de4667248575d1372b1c0acf94d83'

$Models = @(
    [pscustomobject]@{Id='qwen3-4b'; Name='Qwen3 4B'; File='Qwen3-4B-Q4_K_M.gguf'; Url='https://huggingface.co/Qwen/Qwen3-4B-GGUF/resolve/main/Qwen3-4B-Q4_K_M.gguf?download=true'; Sha='7485fe6f11af29433bc51cab58009521f205840f5b4ae3a32fa7f92e8534fdf5'; Size=2.5},
    [pscustomobject]@{Id='qwen3-8b'; Name='Qwen3 8B'; File='Qwen3-8B-Q4_K_M.gguf'; Url='https://huggingface.co/Qwen/Qwen3-8B-GGUF/resolve/main/Qwen3-8B-Q4_K_M.gguf?download=true'; Sha='d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785'; Size=5.03},
    [pscustomobject]@{Id='qwen3-14b'; Name='Qwen3 14B'; File='Qwen3-14B-Q4_K_M.gguf'; Url='https://huggingface.co/ggml-org/Qwen3-14B-GGUF/resolve/main/Qwen3-14B-Q4_K_M.gguf?download=true'; Sha='5ff1fe7a07aebc8d090682d01b17cf268a1b4680c6477050ce75a600aecb9efb'; Size=9.0},
    [pscustomobject]@{Id='qwen3-30b-a3b'; Name='Qwen3 30B-A3B'; File='Qwen3-30B-A3B-Q4_K_M.gguf'; Url='https://huggingface.co/ggml-org/Qwen3-30B-A3B-GGUF/resolve/main/Qwen3-30B-A3B-Q4_K_M.gguf?download=true'; Sha='642a1eda167db5f55de673cbd19fd8b4aac69c766d01bdbeaf5820f457c7790d'; Size=18.6}
)

$AIRoot = Join-Path $env:LOCALAPPDATA 'MarketplacePoster\AI'
$RuntimeDir = Join-Path $AIRoot 'runtime'
$CpuBackupDir = Join-Path $AIRoot 'runtime-cpu'
$ModelDir = Join-Path $AIRoot 'models'
$TempDir = Join-Path $env:TEMP ('MarketplacePosterInstall-' + [guid]::NewGuid().ToString('N'))
$FailureMarker = Join-Path $AIRoot 'install-failed.txt'
New-Item -ItemType Directory -Force -Path $AIRoot,$RuntimeDir,$CpuBackupDir,$ModelDir,$TempDir | Out-Null
Remove-Item -Force $FailureMarker -ErrorAction SilentlyContinue

if ($Repair) {
    Write-Host 'Repair mode: refreshing the local AI runtime and validating the model...'
    try {
        Get-CimInstance Win32_Process -Filter "Name='llama-server.exe'" -ErrorAction SilentlyContinue |
            Where-Object { $_.ExecutablePath -like ($AIRoot + '*') -or $_.CommandLine -like ('*' + $AIRoot + '*') } |
            ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
    } catch {}
    Remove-Item -Recurse -Force $RuntimeDir,$CpuBackupDir -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $RuntimeDir,$CpuBackupDir | Out-Null
}

function Download-File([string]$Url,[string]$Destination,[string]$Label) {
    Write-Host "Downloading $Label..."
    $part = $Destination + '.part'
    $curl = Get-Command curl.exe -ErrorAction SilentlyContinue

    # curl: resume partial downloads and retry transient GitHub/Hugging Face disconnects.
    if ($curl) {
        for ($attempt = 1; $attempt -le 6; $attempt++) {
            Write-Host ("  curl attempt {0}/6" -f $attempt)
            & $curl.Source -L --fail --retry 5 --retry-delay 3 --connect-timeout 30 --continue-at - --progress-bar -o $part $Url
            if ($LASTEXITCODE -eq 0 -and (Test-Path $part) -and ((Get-Item $part).Length -gt 0)) {
                Move-Item -Force $part $Destination
                return
            }
            Start-Sleep -Seconds ([Math]::Min(5 * $attempt, 20))
        }
    }

    # PowerShell fallback. It restarts this one file if curl could not complete it.
    Write-Warning "curl could not finish $Label. Trying PowerShell download fallback..."
    Remove-Item -Force $part -ErrorAction SilentlyContinue
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        try {
            Write-Host ("  PowerShell attempt {0}/3" -f $attempt)
            Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $part -TimeoutSec 0
            if ((Test-Path $part) -and ((Get-Item $part).Length -gt 0)) {
                Move-Item -Force $part $Destination
                return
            }
        } catch {
            Write-Warning $_.Exception.Message
            Start-Sleep -Seconds (5 * $attempt)
        }
    }

    throw "Download failed after retries: $Label"
}

function Verify-SHA256([string]$Path,[string]$Expected,[string]$Label) {
    $actual = (Get-FileHash -Algorithm SHA256 -Path $Path).Hash.ToLowerInvariant()
    if ($actual -ne $Expected.ToLowerInvariant()) {
        throw "SHA-256 verification failed for $Label. Expected $Expected, got $actual"
    }
    Write-Host "Verified $Label SHA-256."
}

function Expand-Llama([string]$Zip,[string]$Destination) {
    $extract = Join-Path $TempDir ([guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force -Path $extract | Out-Null
    Expand-Archive -LiteralPath $Zip -DestinationPath $extract -Force
    $server = Get-ChildItem -Path $extract -Filter 'llama-server.exe' -Recurse | Select-Object -First 1
    if (-not $server) { throw 'llama-server.exe was not found in llama.cpp archive.' }
    Remove-Item -Recurse -Force $Destination -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    Copy-Item -Path (Join-Path $server.Directory.FullName '*') -Destination $Destination -Recurse -Force
}

function Get-Hardware {
    $ram = 8
    try { $ram = [int][math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB) } catch {}
    $gpuName = ''
    $vram = 0
    try {
        $g = Get-CimInstance Win32_VideoController | Sort-Object AdapterRAM -Descending | Select-Object -First 1
        $gpuName = [string]$g.Name
        if ($g.AdapterRAM) { $vram = [int][math]::Round([double]$g.AdapterRAM / 1GB) }
    } catch {}
    [pscustomobject]@{RamGB=$ram; GPU=$gpuName; VRAMGB=$vram}
}

function Choose-Model($h) {
    if ($h.RamGB -ge 56 -or ($h.RamGB -ge 40 -and $h.VRAMGB -ge 16)) { return $Models[3] }
    if ($h.RamGB -ge 28 -or ($h.RamGB -ge 24 -and $h.VRAMGB -ge 10)) { return $Models[2] }
    if ($h.RamGB -ge 16) { return $Models[1] }
    return $Models[0]
}

function Can-UseVulkan($h) {
    if (-not $h.GPU) { return $false }
    return $h.GPU -match '(?i)NVIDIA|AMD|Radeon|Intel.*Arc'
}

function Install-Runtime($h) {
    Write-Host ''
    Write-Host '--- llama.cpp runtime ---'
    $useGPU = Can-UseVulkan $h
    $cpuExe = Join-Path $CpuBackupDir 'llama-server.exe'

    if ($useGPU) {
        # On capable PCs, install the primary GPU runtime first so an optional
        # CPU-backup download can never block the whole installation.
        $runtimeExe = Join-Path $RuntimeDir 'llama-server.exe'
        if (-not (Test-Path $runtimeExe)) {
            $vkZip = Join-Path $TempDir 'llama-vulkan.zip'
            Download-File $VulkanUrl $vkZip 'llama.cpp Vulkan GPU runtime'
            Verify-SHA256 $vkZip $VulkanSha 'llama.cpp Vulkan runtime'
            Expand-Llama $vkZip $RuntimeDir
        }
        Write-Host ("GPU runtime selected: " + $h.GPU)

        # CPU fallback is useful, but optional on a GPU-capable PC.
        if (-not (Test-Path $cpuExe)) {
            try {
                $cpuZip = Join-Path $TempDir 'llama-cpu.zip'
                Download-File $CpuUrl $cpuZip 'llama.cpp CPU fallback runtime'
                Verify-SHA256 $cpuZip $CpuSha 'llama.cpp CPU runtime'
                Expand-Llama $cpuZip $CpuBackupDir
            } catch {
                Write-Warning ('CPU fallback could not be installed right now. GPU AI will still work. Use Start Menu -> Marketplace Poster -> Install or Repair Smart AI later if you want the fallback. ' + $_.Exception.Message)
            }
        }
    } else {
        # CPU-only machines require the CPU runtime, so failure here is fatal.
        if (-not (Test-Path $cpuExe)) {
            $cpuZip = Join-Path $TempDir 'llama-cpu.zip'
            Download-File $CpuUrl $cpuZip 'llama.cpp CPU runtime'
            Verify-SHA256 $cpuZip $CpuSha 'llama.cpp CPU runtime'
            Expand-Llama $cpuZip $CpuBackupDir
        }
        Remove-Item -Recurse -Force $RuntimeDir -ErrorAction SilentlyContinue
        New-Item -ItemType Directory -Force -Path $RuntimeDir | Out-Null
        Copy-Item -Path (Join-Path $CpuBackupDir '*') -Destination $RuntimeDir -Recurse -Force
        Write-Host 'CPU runtime selected.'
    }
    return $useGPU
}

function Install-RecommendedModel($h) {
    $model = Choose-Model $h
    Write-Host ''
    Write-Host ("Hardware: RAM {0}GB | GPU {1} | VRAM {2}GB" -f $h.RamGB, ($(if($h.GPU){$h.GPU}else{'not detected'})), $h.VRAMGB)
    Write-Host ("Recommended model: {0} (~{1}GB)" -f $model.Name, $model.Size)
    $modelPath = Join-Path $ModelDir $model.File
    $need = $true
    if (Test-Path $modelPath) {
        $existing = (Get-FileHash -Algorithm SHA256 -Path $modelPath).Hash.ToLowerInvariant()
        if ($existing -eq $model.Sha) { $need = $false; Write-Host 'Recommended model already installed and verified.' }
        else { Remove-Item -Force $modelPath }
    }
    if ($need) {
        $tmpModel = Join-Path $TempDir ($model.File + '.download')
        Download-File $model.Url $tmpModel ($model.Name + ' model')
        Verify-SHA256 $tmpModel $model.Sha $model.Name
        Move-Item -Force $tmpModel $modelPath
    }
    return $model
}

function Install-AI {
    Write-Host ''
    Write-Host '--- Smart Local AI ---'
    $h = Get-Hardware
    $gpuRuntime = Install-Runtime $h
    $model = Install-RecommendedModel $h
    $cpuBackup = Join-Path $CpuBackupDir 'llama-server.exe'
    $marker = [ordered]@{
        runtime = $(if($gpuRuntime){'llama.cpp b10516 Windows Vulkan'}else{'llama.cpp b10516 Windows CPU'})
        gpu_runtime = [bool]$gpuRuntime
        cpu_backup_path = $cpuBackup
        model = $model.File
        model_id = $model.Id
        model_sha256 = $model.Sha
        local_api = 'http://127.0.0.1:12345'
        ram_gb = $h.RamGB
        gpu = $h.GPU
        vram_gb = $h.VRAMGB
        installed_at = (Get-Date).ToUniversalTime().ToString('o')
    } | ConvertTo-Json
    [IO.File]::WriteAllText((Join-Path $AIRoot 'ai-installed.json'), $marker, (New-Object Text.UTF8Encoding($false)))
    Remove-Item -Force $FailureMarker -ErrorAction SilentlyContinue
    Write-Host 'Smart Local AI is ready.'
}

try {
    Write-Host ''
    Write-Host 'Installing the recommended Smart Local AI...'
    Install-AI
    Write-Host ''
    Write-Host 'Smart Local AI installation completed.' -ForegroundColor Green
    Start-Sleep -Seconds 2
} catch {
    $msg = $_.Exception.Message
    try { [IO.File]::WriteAllText($FailureMarker, $msg, (New-Object Text.UTF8Encoding($false))) } catch {}
    Write-Host ''
    Write-Host ('AI installation could not be completed: ' + $msg) -ForegroundColor Yellow
    Write-Host 'Marketplace Poster itself is already installed and can be used.' -ForegroundColor Green
    Write-Host 'Use Repair AI from Marketplace Poster settings. If Repair AI also fails, contact support.'
    if (-not $Silent) {
        try {
            Add-Type -AssemblyName PresentationFramework -ErrorAction SilentlyContinue
            [System.Windows.MessageBox]::Show("Marketplace Poster is installed.`n`nSmart Local AI could not be completed right now:`n" + $msg + "`n`nUse Repair AI from the app settings. If it still fails, contact support.", 'Marketplace Poster', 'OK', 'Warning') | Out-Null
        } catch {}
        Start-Sleep -Seconds 3
    }
    exit 1
} finally {
    Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
}
