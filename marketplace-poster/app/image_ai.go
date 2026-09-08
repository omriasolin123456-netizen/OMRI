package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	imageAIRuntimeURL = "https://github.com/leejet/stable-diffusion.cpp/releases/download/master-846-d8fb10c/sd-master-d8fb10c-bin-win-cpu-x64.zip"
	imageAIRuntimeSHA = "1f759b875269c1a53fc844daee87c4c39639f73dfe39de50ee11ff41e0bd8785"
	imageAIModelURL   = "https://huggingface.co/QuantStack/Qwen-Image-Edit-2509-GGUF/resolve/main/Qwen-Image-Edit-2509-Q2_K.gguf?download=true"
	imageAIModelSHA   = "8d65b6f3b02c93ad5163b9bb7580ae6dd4cee776224b8d64528459c1e197fe93"
	imageAIVAEURL     = "https://huggingface.co/Comfy-Org/Qwen-Image_ComfyUI/resolve/main/split_files/vae/qwen_image_vae.safetensors?download=true"
	imageAIVAESHA     = "a70580f0213e67967ee9c95f05bb400e8fb08307e017a924bf3441223e023d1f"
	imageAILLMURL     = "https://huggingface.co/ggml-org/Qwen2.5-VL-7B-Instruct-GGUF/resolve/main/Qwen2.5-VL-7B-Instruct-Q4_K_M.gguf?download=true"
	imageAILLMSHA     = "9258bf05b12686d097ff3b6b18d968ab393649780aa2b3cd67fec43d50554392"
	imageAIVisionURL  = "https://huggingface.co/ggml-org/Qwen2.5-VL-7B-Instruct-GGUF/resolve/main/mmproj-Qwen2.5-VL-7B-Instruct-Q8_0.gguf?download=true"
	imageAIVisionSHA  = "2ddb555391bae966e412deab9e07b58afa18bcc06930ba0f1c78a3695ab9e506"
)

type ImageAIStatus struct {
	Enabled    bool    `json:"enabled"`
	Eligible   bool    `json:"eligible"`
	Ready      bool    `json:"ready"`
	Installing bool    `json:"installing"`
	Runtime    string  `json:"runtime,omitempty"`
	Model      string  `json:"model,omitempty"`
	Message    string  `json:"message"`
	PackSizeGB float64 `json:"pack_size_gb"`
}

func imageAIRoot() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserCacheDir()
	}
	return filepath.Join(base, "MarketplacePoster", "AI", "image")
}

func defaultImageAIPaths() (runtimePath, modelPath, vaePath, llmPath, visionPath string) {
	root := imageAIRoot()
	runtimePath = filepath.Join(root, "runtime", "sd-cli.exe")
	modelPath = filepath.Join(root, "models", "Qwen-Image-Edit-2509-Q2_K.gguf")
	vaePath = filepath.Join(root, "models", "qwen_image_vae.safetensors")
	llmPath = filepath.Join(root, "models", "Qwen2.5-VL-7B-Instruct-Q4_K_M.gguf")
	visionPath = filepath.Join(root, "models", "mmproj-Qwen2.5-VL-7B-Instruct-Q8_0.gguf")
	return
}

func resolvedImageAIPaths(settings Settings) (runtimePath, modelPath, vaePath, llmPath, visionPath string) {
	dr, dm, dv, dl, dvis := defaultImageAIPaths()
	runtimePath = strings.TrimSpace(settings.ImageAIRuntimePath)
	if runtimePath == "" {
		runtimePath = dr
	}
	modelPath = strings.TrimSpace(settings.ImageAIModelPath)
	if modelPath == "" {
		modelPath = dm
	}
	vaePath = strings.TrimSpace(settings.ImageAIVAEPath)
	if vaePath == "" {
		vaePath = dv
	}
	llmPath = strings.TrimSpace(settings.ImageAILLMPath)
	if llmPath == "" {
		llmPath = dl
	}
	visionPath = strings.TrimSpace(settings.ImageAILLMVisionPath)
	if visionPath == "" {
		visionPath = dvis
	}
	return
}

func imageAIStatus(settings Settings) ImageAIStatus {
	h := detectHardware()
	runtimePath, modelPath, vaePath, llmPath, visionPath := resolvedImageAIPaths(settings)
	st := ImageAIStatus{Enabled: settings.ImageAIEnabled, Eligible: h.ImageAIEligible, Runtime: runtimePath, Model: modelPath, PackSizeGB: 13.0}
	if !st.Eligible {
		st.Message = "החומרה אינה מומלצת ל-AI תמונה מקומי. רנדומייז רגיל של תמונות ימשיך לעבוד."
		return st
	}
	files := []string{runtimePath, modelPath, vaePath, llmPath, visionPath}
	st.Ready = true
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			st.Ready = false
			break
		}
	}
	logPath := filepath.Join(imageAIRoot(), "image-ai-install.log")
	if !st.Ready {
		if b, err := os.ReadFile(logPath); err == nil {
			txt := string(b)
			if strings.Contains(txt, "START") && !strings.Contains(txt, "DONE") && !strings.Contains(txt, "FAILED") {
				st.Installing = true
				st.Message = "Image AI Pack בהתקנה. החבילה גדולה (~13GB); אפשר להמשיך לעבוד בזמן ההורדה."
				return st
			}
		}
	}
	if st.Ready {
		st.Message = "AI תמונה מוכן: Qwen Image Edit 2509 מקומי"
	} else {
		st.Message = "המחשב מתאים. אפשר להתקין Image AI Pack (~13GB) בלחיצה אחת."
	}
	return st
}

func launchImageAIInstall(settings Settings) (Settings, error) {
	if runtime.GOOS != "windows" {
		return settings, fmt.Errorf("Image AI Pack זמין כרגע ב-Windows")
	}
	h := detectHardware()
	if !h.ImageAIEligible {
		return settings, fmt.Errorf("החומרה אינה מומלצת ל-Image AI Pack")
	}
	root := imageAIRoot()
	runtimeDir := filepath.Join(root, "runtime")
	modelDir := filepath.Join(root, "models")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return settings, err
	}
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		return settings, err
	}
	runtimePath, modelPath, vaePath, llmPath, visionPath := defaultImageAIPaths()
	logPath := filepath.Join(root, "image-ai-install.log")
	tempDir := filepath.Join(root, "tmp")
	zipPath := filepath.Join(tempDir, "sd-runtime.zip")
	script := fmt.Sprintf(`$ErrorActionPreference='Stop'; New-Item -ItemType Directory -Force -Path '%s','%s','%s'|Out-Null; 'START'|Set-Content -Encoding UTF8 '%s'; function dl($u,$d,$sha,$name){ if(Test-Path $d){$h=(Get-FileHash $d -Algorithm SHA256).Hash.ToLower(); if($h -eq $sha){ "$name OK"|Add-Content -Encoding UTF8 '%s'; return }; Remove-Item $d -Force }; "$name downloading"|Add-Content -Encoding UTF8 '%s'; $t=$d+'.download'; & curl.exe -L --fail --retry 4 --retry-delay 3 -o $t $u; if($LASTEXITCODE -ne 0){throw "download failed: $name"}; $h=(Get-FileHash $t -Algorithm SHA256).Hash.ToLower(); if($h -ne $sha){Remove-Item $t -Force; throw "SHA mismatch: $name"}; Move-Item $t $d -Force }; try { dl '%s' '%s' '%s' 'Qwen Image Edit model'; dl '%s' '%s' '%s' 'Qwen Image VAE'; dl '%s' '%s' '%s' 'Qwen2.5-VL encoder'; dl '%s' '%s' '%s' 'Qwen2.5-VL vision'; if(-not (Test-Path '%s')){ dl '%s' '%s' '%s' 'stable-diffusion.cpp runtime'; $x='%s'; Remove-Item -Recurse -Force $x -ErrorAction SilentlyContinue; New-Item -ItemType Directory -Force -Path $x|Out-Null; Expand-Archive -LiteralPath '%s' -DestinationPath $x -Force; $exe=Get-ChildItem $x -Filter 'sd-cli.exe' -Recurse|Select-Object -First 1; if(-not $exe){throw 'sd-cli.exe missing'}; Remove-Item -Recurse -Force '%s' -ErrorAction SilentlyContinue; New-Item -ItemType Directory -Force -Path '%s'|Out-Null; Copy-Item (Join-Path $exe.Directory.FullName '*') '%s' -Recurse -Force }; 'DONE'|Add-Content -Encoding UTF8 '%s' } catch { ('FAILED: '+$_.Exception.Message)|Add-Content -Encoding UTF8 '%s'; throw }`,
		psSingle(root), psSingle(runtimeDir), psSingle(modelDir), psSingle(logPath), psSingle(logPath), psSingle(logPath),
		psSingle(imageAIModelURL), psSingle(modelPath), imageAIModelSHA,
		psSingle(imageAIVAEURL), psSingle(vaePath), imageAIVAESHA,
		psSingle(imageAILLMURL), psSingle(llmPath), imageAILLMSHA,
		psSingle(imageAIVisionURL), psSingle(visionPath), imageAIVisionSHA,
		psSingle(runtimePath), psSingle(imageAIRuntimeURL), psSingle(zipPath), imageAIRuntimeSHA,
		psSingle(filepath.Join(tempDir, "runtime-extract")), psSingle(zipPath), psSingle(runtimeDir), psSingle(runtimeDir), psSingle(runtimeDir), psSingle(logPath), psSingle(logPath))
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", script)
	if err := cmd.Start(); err != nil {
		return settings, err
	}
	settings.ImageAIEnabled = true
	settings.ImageAIRuntimePath = runtimePath
	settings.ImageAIModelPath = modelPath
	settings.ImageAIVAEPath = vaePath
	settings.ImageAILLMPath = llmPath
	settings.ImageAILLMVisionPath = visionPath
	return settings, nil
}

// generateProductImageVariant keeps the real product identity. It is for presentation
// variations (background, lighting and framing), not for inventing product features.
func generateProductImageVariant(settings Settings, inputPath, outputPath, hint string) error {
	st := imageAIStatus(settings)
	if !st.Enabled || !st.Ready {
		return fmt.Errorf("AI תמונה אינו מוכן")
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("AI תמונה אוטומטי נתמך כרגע ב-Windows")
	}
	runtimePath, modelPath, vaePath, llmPath, visionPath := resolvedImageAIPaths(settings)
	prompt := "Create a realistic marketplace product-photo variation. Preserve the exact product identity, geometry, color, visible condition, logos, text and included parts. Do not invent accessories, features, damage or improvements to the actual product. Change only background, lighting, camera framing and presentation while keeping the product truthful. " + strings.TrimSpace(hint)
	args := []string{"--diffusion-model", modelPath, "--vae", vaePath, "--llm", llmPath, "--llm_vision", visionPath, "--cfg-scale", "2.5", "--sampling-method", "euler", "--offload-to-cpu", "--diffusion-fa", "--flow-shift", "3", "-r", inputPath, "-o", outputPath, "-p", prompt}
	cmd := exec.Command(runtimePath, args...)
	cmd.Dir = filepath.Dir(runtimePath)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("image AI failed: %v: %s", err, strings.TrimSpace(string(b)))
	}
	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("image AI did not create an output")
	}
	return nil
}
