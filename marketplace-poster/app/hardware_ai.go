package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type AIModelProfile struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Filename       string  `json:"filename"`
	URL            string  `json:"url"`
	SHA256         string  `json:"sha256"`
	SizeGB         float64 `json:"size_gb"`
	MinRAMGB       int     `json:"min_ram_gb"`
	RecommendedRAM int     `json:"recommended_ram_gb"`
	Quality        string  `json:"quality"`
}

type HardwareInfo struct {
	OS               string           `json:"os"`
	Arch             string           `json:"arch"`
	CPU              string           `json:"cpu,omitempty"`
	RAMGB            int              `json:"ram_gb"`
	GPU              string           `json:"gpu,omitempty"`
	VRAMGB           int              `json:"vram_gb"`
	RecommendedModel string           `json:"recommended_model"`
	ImageAIEligible  bool             `json:"image_ai_eligible"`
	Reason           string           `json:"reason"`
	Models           []AIModelProfile `json:"models"`
}

var aiModelProfiles = []AIModelProfile{
	{ID: "qwen3-4b", Name: "Qwen3 4B — מהיר", Filename: "Qwen3-4B-Q4_K_M.gguf", URL: "https://huggingface.co/Qwen/Qwen3-4B-GGUF/resolve/main/Qwen3-4B-Q4_K_M.gguf?download=true", SHA256: "7485fe6f11af29433bc51cab58009521f205840f5b4ae3a32fa7f92e8534fdf5", SizeGB: 2.5, MinRAMGB: 8, RecommendedRAM: 12, Quality: "טוב"},
	{ID: "qwen3-8b", Name: "Qwen3 8B — איכות גבוהה", Filename: "Qwen3-8B-Q4_K_M.gguf", URL: "https://huggingface.co/Qwen/Qwen3-8B-GGUF/resolve/main/Qwen3-8B-Q4_K_M.gguf?download=true", SHA256: "d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785", SizeGB: 5.03, MinRAMGB: 16, RecommendedRAM: 20, Quality: "גבוה"},
	{ID: "qwen3-14b", Name: "Qwen3 14B — מכירתי חזק", Filename: "Qwen3-14B-Q4_K_M.gguf", URL: "https://huggingface.co/ggml-org/Qwen3-14B-GGUF/resolve/main/Qwen3-14B-Q4_K_M.gguf?download=true", SHA256: "5ff1fe7a07aebc8d090682d01b17cf268a1b4680c6477050ce75a600aecb9efb", SizeGB: 9.0, MinRAMGB: 24, RecommendedRAM: 32, Quality: "גבוה מאוד"},
	{ID: "qwen3-30b-a3b", Name: "Qwen3 30B-A3B — Maximum Quality", Filename: "Qwen3-30B-A3B-Q4_K_M.gguf", URL: "https://huggingface.co/ggml-org/Qwen3-30B-A3B-GGUF/resolve/main/Qwen3-30B-A3B-Q4_K_M.gguf?download=true", SHA256: "642a1eda167db5f55de673cbd19fd8b4aac69c766d01bdbeaf5820f457c7790d", SizeGB: 18.6, MinRAMGB: 48, RecommendedRAM: 64, Quality: "מקסימלית"},
}

func profileByID(id string) (AIModelProfile, bool) {
	for _, p := range aiModelProfiles {
		if p.ID == id || p.Filename == id {
			return p, true
		}
	}
	return AIModelProfile{}, false
}

func profileByFilename(name string) (AIModelProfile, bool) {
	for _, p := range aiModelProfiles {
		if strings.EqualFold(p.Filename, filepath.Base(name)) {
			return p, true
		}
	}
	return AIModelProfile{}, false
}

func detectHardware() HardwareInfo {
	h := HardwareInfo{OS: runtime.GOOS, Arch: runtime.GOARCH, Models: aiModelProfiles}
	if runtime.GOOS == "windows" {
		h.RAMGB = powershellInt(`$m=(Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory; [math]::Round($m/1GB)`)
		h.CPU = powershellText(`(Get-CimInstance Win32_Processor | Select-Object -First 1 -ExpandProperty Name)`)
		h.GPU = powershellText(`(Get-CimInstance Win32_VideoController | Sort-Object AdapterRAM -Descending | Select-Object -First 1 -ExpandProperty Name)`)
		h.VRAMGB = powershellInt(`$g=Get-CimInstance Win32_VideoController | Sort-Object AdapterRAM -Descending | Select-Object -First 1; if($g.AdapterRAM){[math]::Round($g.AdapterRAM/1GB)}else{0}`)
	} else {
		h.RAMGB = linuxRAMGB()
		h.CPU = runtime.GOARCH
	}
	if h.RAMGB <= 0 {
		h.RAMGB = 8
	}
	h.RecommendedModel = recommendModel(h).ID
	h.ImageAIEligible = h.VRAMGB >= 12 || h.RAMGB >= 48
	if h.ImageAIEligible {
		h.Reason = "המחשב מתאים גם למנוע וריאציות תמונה מקומי. המערכת תשמור על זהות המוצר ולא תמציא תכונות."
	} else {
		h.Reason = "AI טקסט יותאם אוטומטית לזיכרון. AI תמונה יישאר כבוי כדי לא להעמיס על המחשב."
	}
	return h
}

func recommendModel(h HardwareInfo) AIModelProfile {
	if h.RAMGB >= 56 || (h.RAMGB >= 40 && h.VRAMGB >= 16) {
		return aiModelProfiles[3]
	}
	if h.RAMGB >= 28 || (h.RAMGB >= 24 && h.VRAMGB >= 10) {
		return aiModelProfiles[2]
	}
	if h.RAMGB >= 16 {
		return aiModelProfiles[1]
	}
	return aiModelProfiles[0]
}

func powershellText(script string) string {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	b, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func powershellInt(script string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(powershellText(script)))
	return v
}

func linuxRAMGB() int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return int((kb + 1024*1024 - 1) / (1024 * 1024))
			}
		}
	}
	return 0
}

func modelsRoot(settings Settings) string {
	root := strings.TrimSpace(settings.LocalAIPath)
	if root == "" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base, _ = os.UserCacheDir()
		}
		root = filepath.Join(base, "MarketplacePoster", "AI")
	}
	return filepath.Join(root, "models")
}

func installedModels(settings Settings) map[string]bool {
	out := map[string]bool{}
	for _, p := range aiModelProfiles {
		st, err := os.Stat(filepath.Join(modelsRoot(settings), p.Filename))
		out[p.ID] = err == nil && st.Size() > 100*1024*1024
	}
	return out
}

func launchModelInstall(settings Settings, profile AIModelProfile) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("התקנת מודלים אוטומטית זמינה כרגע ב-Windows")
	}
	root := modelsRoot(settings)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(root, profile.Filename)
	logPath := filepath.Join(root, "model-install.log")
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'; $u='%s'; $d='%s'; $t=$d+'.download'; $log='%s'; "Downloading %s" | Set-Content -Encoding UTF8 $log; Invoke-WebRequest -Uri $u -OutFile $t -UseBasicParsing; $h=(Get-FileHash $t -Algorithm SHA256).Hash.ToLower(); if($h -ne '%s'){Remove-Item $t -Force; throw 'SHA-256 mismatch'}; Move-Item $t $d -Force; 'DONE' | Add-Content -Encoding UTF8 $log`, psSingle(profile.URL), psSingle(dst), psSingle(logPath), psSingle(profile.Name), strings.ToLower(profile.SHA256))
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", script)
	return cmd.Start()
}

func launchAllModelsInstall(settings Settings) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("התקנת מודלים אוטומטית זמינה כרגע ב-Windows")
	}
	root := modelsRoot(settings)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(root, "model-install-all.log")
	var lines []string
	lines = append(lines, "$ErrorActionPreference='Stop'", fmt.Sprintf("'Starting all text models' | Set-Content -Encoding UTF8 '%s'", psSingle(logPath)))
	for _, profile := range aiModelProfiles {
		dst := filepath.Join(root, profile.Filename)
		lines = append(lines, fmt.Sprintf(`$d='%s'; $u='%s'; if(Test-Path $d){$h=(Get-FileHash $d -Algorithm SHA256).Hash.ToLower(); if($h -eq '%s'){ '%s already OK' | Add-Content -Encoding UTF8 '%s'; } else { Remove-Item $d -Force }}; if(-not (Test-Path $d)){ '%s' | Add-Content -Encoding UTF8 '%s'; $t=$d+'.download'; Invoke-WebRequest -Uri $u -OutFile $t -UseBasicParsing; $h=(Get-FileHash $t -Algorithm SHA256).Hash.ToLower(); if($h -ne '%s'){Remove-Item $t -Force; throw 'SHA-256 mismatch'}; Move-Item $t $d -Force }`, psSingle(dst), psSingle(profile.URL), strings.ToLower(profile.SHA256), psSingle(profile.Name), psSingle(logPath), psSingle("Downloading "+profile.Name), psSingle(logPath), strings.ToLower(profile.SHA256)))
	}
	lines = append(lines, fmt.Sprintf("'DONE' | Add-Content -Encoding UTF8 '%s'", psSingle(logPath)))
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", strings.Join(lines, "; "))
	return cmd.Start()
}

func psSingle(s string) string { return strings.ReplaceAll(s, "'", "''") }

func effectiveTextModel(settings Settings) AIModelProfile {
	if settings.AIModelMode == "manual" {
		if p, ok := profileByFilename(settings.LocalAIModel); ok {
			return p
		}
		if p, ok := profileByID(settings.LocalAIModel); ok {
			return p
		}
	}
	h := detectHardware()
	rec := recommendModel(h)
	installed := installedModels(settings)
	// Choose the strongest installed model that does not exceed the recommendation.
	best := aiModelProfiles[0]
	for _, p := range aiModelProfiles {
		if p.SizeGB <= rec.SizeGB && installed[p.ID] {
			best = p
		}
	}
	if installed[best.ID] {
		return best
	}
	return rec
}
