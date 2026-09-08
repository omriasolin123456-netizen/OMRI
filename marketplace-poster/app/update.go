package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type UpdateManifest struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	SHA256      string `json:"sha256,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

func fetchUpdateManifest(raw string) (UpdateManifest, error) {
	var m UpdateManifest
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return m, err
	}
	if u.Scheme != "https" {
		return m, fmt.Errorf("update manifest must use https")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return m, fmt.Errorf("update server returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return m, err
	}
	if strings.TrimSpace(m.Version) == "" || strings.TrimSpace(m.DownloadURL) == "" {
		return m, fmt.Errorf("invalid update manifest")
	}
	d, err := url.Parse(m.DownloadURL)
	if err != nil || d.Scheme != "https" {
		return m, fmt.Errorf("download_url must use https")
	}
	return m, nil
}

func versionGreater(a, b string) bool {
	parse := func(v string) []int {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		parts := strings.Split(v, ".")
		out := make([]int, 3)
		for i := 0; i < len(out) && i < len(parts); i++ {
			n, _ := strconv.Atoi(strings.TrimFunc(parts[i], func(r rune) bool { return r < '0' || r > '9' }))
			out[i] = n
		}
		return out
	}
	aa, bb := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		if aa[i] > bb[i] {
			return true
		}
		if aa[i] < bb[i] {
			return false
		}
	}
	return false
}

func downloadUpdate(dataDir string, m UpdateManifest) (string, error) {
	if strings.TrimSpace(m.SHA256) == "" {
		return "", fmt.Errorf("manifest must include sha256 before an update can be downloaded")
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(m.DownloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	dir := filepath.Join(dataDir, "updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "MarketplacePoster-"+strings.TrimPrefix(m.Version, "v")+".exe")
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, cpErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 300<<20))
	closeErr := f.Close()
	if cpErr != nil {
		_ = os.Remove(tmp)
		return "", cpErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(m.SHA256)) {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("update checksum mismatch")
	}
	_ = os.Remove(path)
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

func scheduleSelfUpdate(newExe string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("self update is supported on Windows only")
	}
	currentExe, err := os.Executable()
	if err != nil {
		return err
	}
	currentExe, _ = filepath.Abs(currentExe)
	newExe, _ = filepath.Abs(newExe)
	if !strings.EqualFold(filepath.Ext(currentExe), ".exe") {
		return fmt.Errorf("current program is not a Windows executable")
	}
	if _, err := os.Stat(newExe); err != nil {
		return fmt.Errorf("downloaded update not found: %w", err)
	}

	script := filepath.Join(filepath.Dir(newExe), "apply-update.ps1")
	ps := `param([int]$ProcessId,[string]$CurrentExe,[string]$NewExe)
$ErrorActionPreference = 'Stop'

# Marketplace Poster may be installed under Program Files. If this update helper
# is not elevated, relaunch only the update step with the normal Windows UAC
# consent dialog. The main application does not bypass UAC.
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
$isAdmin = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
  $argLine = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "' + $PSCommandPath + '" -ProcessId ' + $ProcessId + ' -CurrentExe "' + $CurrentExe + '" -NewExe "' + $NewExe + '"'
  Start-Process -FilePath 'powershell.exe' -Verb RunAs -WindowStyle Hidden -ArgumentList $argLine
  exit 0
}

try { Wait-Process -Id $ProcessId -Timeout 45 -ErrorAction SilentlyContinue } catch {}
$ok = $false
for ($i = 0; $i -lt 60; $i++) {
  try {
    Copy-Item -LiteralPath $NewExe -Destination $CurrentExe -Force
    $ok = $true
    break
  } catch {
    Start-Sleep -Milliseconds 500
  }
}
if (-not $ok) { exit 2 }
Start-Process -FilePath $CurrentExe
Start-Sleep -Milliseconds 800
Remove-Item -LiteralPath $NewExe -Force -ErrorAction SilentlyContinue
`
	if err := os.WriteFile(script, []byte(ps), 0o600); err != nil {
		return err
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", script,
		"-ProcessId", strconv.Itoa(os.Getpid()), "-CurrentExe", currentExe, "-NewExe", newExe)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}
