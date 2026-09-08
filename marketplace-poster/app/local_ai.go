package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	defaultLocalAIBaseURL = "http://127.0.0.1:12345"
	defaultLocalAIModel   = "Qwen3-4B-Q4_K_M.gguf"
)

var (
	localAIStartMu   sync.Mutex
	localAIProcessMu sync.Mutex
	localAIProcess   *os.Process
)

type LocalAIStatus struct {
	Enabled         bool   `json:"enabled"`
	Installed       bool   `json:"installed"`
	Installing      bool   `json:"installing"`
	Running         bool   `json:"running"`
	State           string `json:"state"`
	RepairAvailable bool   `json:"repair_available"`
	BaseURL         string `json:"base_url"`
	RuntimePath     string `json:"runtime_path"`
	ModelPath       string `json:"model_path"`
	Model           string `json:"model"`
	Message         string `json:"message,omitempty"`
	LastError       string `json:"last_error,omitempty"`
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Stream      bool                `json:"stream"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func localAIBaseURL(settings Settings) string {
	v := strings.TrimSpace(settings.LocalAIBaseURL)
	if v == "" {
		return defaultLocalAIBaseURL
	}
	return strings.TrimRight(v, "/")
}

func localAIPaths(settings Settings) (runtimePath, modelPath string) {
	aiRoot := strings.TrimSpace(settings.LocalAIPath)
	if aiRoot == "" {
		base := os.Getenv("LOCALAPPDATA")
		if strings.TrimSpace(base) == "" {
			if d, err := os.UserCacheDir(); err == nil {
				base = d
			}
		}
		if strings.TrimSpace(base) == "" {
			base = "."
		}
		aiRoot = filepath.Join(base, "MarketplacePoster", "AI")
	}
	runtimePath = filepath.Join(aiRoot, "runtime", "llama-server.exe")
	modelName := strings.TrimSpace(settings.LocalAIModel)
	if settings.AIModelMode == "auto" {
		modelName = effectiveTextModel(settings).Filename
	}
	if modelName == "" {
		modelName = defaultLocalAIModel
	}
	modelPath = filepath.Join(aiRoot, "models", modelName)
	return
}

func localAIStatus(settings Settings) LocalAIStatus {
	runtimePath, modelPath := localAIPaths(settings)
	st := LocalAIStatus{
		Enabled:     settings.LocalAIEnabled,
		BaseURL:     localAIBaseURL(settings),
		RuntimePath: runtimePath,
		ModelPath:   modelPath,
		Model:       filepath.Base(modelPath),
	}
	_, rerr := os.Stat(runtimePath)
	_, merr := os.Stat(modelPath)
	st.Installed = rerr == nil && merr == nil
	st.Installing = smartAIInstallRunning(settings)
	st.Running = localAIHealth(st.BaseURL)
	st.LastError = readSmartAIFailure(settings)
	switch {
	case st.Running:
		st.State = "ready"
		st.Message = "AI מקומי מוכן"
	case st.Installing:
		st.State = "installing"
		st.Message = "Smart AI מתקין או מתקן את עצמו ברקע — אפשר להמשיך לעבוד"
	case !st.Installed && st.LastError != "":
		st.State = "install_failed"
		st.RepairAvailable = true
		st.Message = "התקנת Smart AI הקודמת לא הושלמה. אפשר לנסות Repair AI."
	case !st.Installed:
		st.State = "not_installed"
		st.Message = "Smart AI עדיין לא מותקן"
	default:
		st.State = "installed_stopped"
		st.RepairAvailable = true
		st.Message = "AI מותקן אך אינו פועל כרגע"
	}
	return st
}

func localAIHealth(baseURL string) bool {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	for _, p := range []string{"/health", "/v1/models"} {
		req, _ := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+p, nil)
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return true
			}
		}
	}
	return false
}

func ensureLocalAI(settings Settings) error {
	if localAIHealth(localAIBaseURL(settings)) {
		return nil
	}
	if !settings.LocalAIAutoStart {
		return fmt.Errorf("AI המקומי אינו פועל")
	}
	runtimePath, modelPath := localAIPaths(settings)
	if _, err := os.Stat(runtimePath); err != nil {
		return fmt.Errorf("מנוע AI מקומי לא נמצא. הרץ את המתקין או התקן מודל מההגדרות")
	}
	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("מודל AI מקומי לא נמצא. התקן את המודל המומלץ מההגדרות")
	}

	localAIStartMu.Lock()
	defer localAIStartMu.Unlock()
	if localAIHealth(localAIBaseURL(settings)) {
		return nil
	}

	u, err := url.Parse(localAIBaseURL(settings))
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("כתובת AI מקומית אינה תקינה")
	}
	port := u.Port()
	if port == "" {
		port = "12345"
	}
	threads := runtime.NumCPU() - 1
	if threads < 2 {
		threads = 2
	}

	startRuntime := func(path string, gpu bool, timeout time.Duration) error {
		args := []string{
			"-m", modelPath,
			"--host", "127.0.0.1",
			"--port", port,
			"-c", "6144",
			"-t", fmt.Sprintf("%d", threads),
			"--parallel", "1",
		}
		if gpu {
			args = append(args, "-ngl", "99")
		}
		cmd := exec.Command(path, args...)
		logPath := filepath.Join(filepath.Dir(filepath.Dir(runtimePath)), "llama-server.log")
		if f, e := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); e == nil {
			cmd.Stdout = f
			cmd.Stderr = f
		}
		if e := cmd.Start(); e != nil {
			return e
		}
		localAIProcessMu.Lock()
		localAIProcess = cmd.Process
		localAIProcessMu.Unlock()
		log.Printf("local AI started pid=%d model=%s runtime=%s gpu=%v", cmd.Process.Pid, modelPath, path, gpu)
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			if localAIHealth(localAIBaseURL(settings)) {
				return nil
			}
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		_ = cmd.Process.Kill()
		localAIProcessMu.Lock()
		if localAIProcess == cmd.Process {
			localAIProcess = nil
		}
		localAIProcessMu.Unlock()
		return fmt.Errorf("runtime did not become ready")
	}

	gpu := localAIRuntimeSupportsGPU(runtimePath)
	if err := startRuntime(runtimePath, gpu, 75*time.Second); err == nil {
		return nil
	} else if gpu {
		if cpuPath := localAICPUBackupPath(runtimePath); cpuPath != "" {
			if _, statErr := os.Stat(cpuPath); statErr == nil {
				log.Printf("GPU AI runtime failed; falling back to CPU runtime")
				if cpuErr := startRuntime(cpuPath, false, 150*time.Second); cpuErr == nil {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("AI המקומי לא הצליח לעלות. בדוק llama-server.log")
}

func stopLocalAI() {
	localAIProcessMu.Lock()
	p := localAIProcess
	localAIProcess = nil
	localAIProcessMu.Unlock()
	if p != nil {
		_ = p.Kill()
	}
}

func localAIRuntimeSupportsGPU(runtimePath string) bool {
	root := filepath.Dir(filepath.Dir(runtimePath))
	b, err := os.ReadFile(filepath.Join(root, "ai-installed.json"))
	if err != nil {
		return false
	}
	var marker struct {
		GPURuntime bool `json:"gpu_runtime"`
	}
	return json.Unmarshal(b, &marker) == nil && marker.GPURuntime
}

func localAICPUBackupPath(runtimePath string) string {
	root := filepath.Dir(filepath.Dir(runtimePath))
	b, err := os.ReadFile(filepath.Join(root, "ai-installed.json"))
	if err != nil {
		return ""
	}
	var marker struct {
		CPUBackupPath string `json:"cpu_backup_path"`
	}
	if json.Unmarshal(b, &marker) != nil || strings.TrimSpace(marker.CPUBackupPath) == "" {
		return ""
	}
	return marker.CPUBackupPath
}

func smartAIFailurePath(settings Settings) string {
	return filepath.Join(smartAIRoot(settings), "install-failed.txt")
}

func readSmartAIFailure(settings Settings) string {
	b, err := os.ReadFile(smartAIFailurePath(settings))
	if err != nil {
		return ""
	}
	msg := strings.TrimSpace(string(b))
	if len(msg) > 600 {
		msg = msg[:600]
	}
	return msg
}

func smartAIRoot(settings Settings) string {
	runtimePath, _ := localAIPaths(settings)
	return filepath.Dir(filepath.Dir(runtimePath))
}

func smartAIInstallLockPath(settings Settings) string {
	return filepath.Join(smartAIRoot(settings), "installing.lock")
}

func smartAIInstallRunning(settings Settings) bool {
	p := smartAIInstallLockPath(settings)
	st, err := os.Stat(p)
	if err != nil {
		return false
	}
	// A stale lock from an interrupted install must not leave the UI stuck forever.
	if time.Since(st.ModTime()) > 8*time.Hour {
		_ = os.Remove(p)
		return false
	}
	return true
}

func smartAIInstallerScript() (scriptPath, appDir string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	appDir = filepath.Dir(exe)
	scriptPath = filepath.Join(appDir, "installer", "INSTALL_AI.ps1")
	if _, err := os.Stat(scriptPath); err != nil {
		return "", "", fmt.Errorf("קובץ התקנת Smart AI לא נמצא. הפעל שוב את MarketplacePoster Setup כדי לבצע תיקון")
	}
	return scriptPath, appDir, nil
}

func launchSmartAIInstall(settings Settings) error {
	return launchSmartAIInstallMode(settings, false)
}

func launchSmartAIRepair(settings Settings) error {
	return launchSmartAIInstallMode(settings, true)
}

func launchSmartAIInstallMode(settings Settings, repair bool) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("התקנת Smart AI אוטומטית זמינה כרגע ב-Windows")
	}
	if smartAIInstallRunning(settings) {
		return fmt.Errorf("Smart AI כבר מתקין או מתקן את עצמו ברקע")
	}
	scriptPath, appDir, err := smartAIInstallerScript()
	if err != nil {
		return err
	}
	root := smartAIRoot(settings)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	lockPath := smartAIInstallLockPath(settings)
	logPath := filepath.Join(root, "install.log")
	_ = os.Remove(smartAIFailurePath(settings))
	if err := os.WriteFile(lockPath, []byte(time.Now().UTC().Format(time.RFC3339)), 0o600); err != nil {
		return err
	}
	repairArg := ""
	if repair {
		repairArg = " -Repair"
	}
	command := fmt.Sprintf(`$ErrorActionPreference='Continue'; try { & '%s' -InstalledAppDir '%s' -Silent%s *> '%s'; if($LASTEXITCODE -ne 0){ exit $LASTEXITCODE } } finally { Remove-Item -LiteralPath '%s' -Force -ErrorAction SilentlyContinue }`, psSingle(scriptPath), psSingle(appDir), repairArg, psSingle(logPath), psSingle(lockPath))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", command)
	if err := cmd.Start(); err != nil {
		_ = os.Remove(lockPath)
		return err
	}
	return nil
}

func normalizedCreativeMode(settings Settings, req aiRequest) string {
	mode := strings.TrimSpace(req.Creativity)
	if mode == "" {
		mode = strings.TrimSpace(settings.AICreativeMode)
	}
	switch mode {
	case "conservative", "balanced", "creative":
		return mode
	default:
		return "creative"
	}
}

func aiVariantCount(settings Settings, req aiRequest) int {
	n := req.Count
	if n <= 0 {
		n = settings.AIVariantCount
	}
	if n < 2 {
		n = 2
	}
	if n > 10 {
		n = 10
	}
	return n
}

func creativeRules(mode string) string {
	switch mode {
	case "conservative":
		return "שנה ניסוח בעדינות בלבד. שמור מבנה דומה למקור והימנע מתוספות סגנוניות מיותרות."
	case "balanced":
		return "כתוב מחדש בצורה טבעית יותר, שנה פתיח וסדר משפטים, והוסף ניסוח שיווקי מתון בלי להוסיף עובדות חדשות."
	default:
		return "היה יצירתי בניסוח: שנה פתיח, סדר משפטים, אוצר מילים וקצב. אפשר להעשיר את הכתיבה מבחינה סגנונית, אבל אסור להמציא עובדות, תכונות, אחריות, מבצעים או נתונים שלא הופיעו במקור."
	}
}

func aiInstruction(settings Settings, req aiRequest) (string, error) {
	mode := normalizedCreativeMode(settings, req)
	count := aiVariantCount(settings, req)
	languageRule := "כתוב בעברית טבעית וברורה. אל תתרגם את הטקסט לשפה אחרת."
	if !settings.AIHebrewOnly && strings.TrimSpace(req.Language) == "auto" {
		languageRule = "ענה בשפת הקלט."
	}
	base := languageRule + " " + creativeRules(mode) + " אתה קופירייטר מכירות מנוסה למודעות יד שנייה ומוצרים. המטרה היא לגרום לקורא להבין מהר מה המוצר, למה הוא מעניין ומה הצעד הבא, בלי קלישאות מוגזמות. שמור במדויק מחיר, מספרי טלפון, מידות, שמות, דגמים, מצב, מיקום וכל עובדה קונקרטית. אל תמציא מידע, אחריות, אביזרים או תכונות שלא נמסרו."
	switch req.Mode {
	case "improve":
		return base + " שפר את הטקסט למודעת Marketplace ברורה ומשכנעת. החזר רק את הטקסט המשופר.", nil
	case "titles":
		return fmt.Sprintf("%s צור %d כותרות קצרות ושונות באמת. כל כותרת בשורה נפרדת, ללא מספור וללא הסברים.", base, count), nil
	case "shorten":
		return base + " קצר את הטקסט בלי לאבד פרטים חשובים. החזר רק את הטקסט המקוצר.", nil
	case "spelling":
		return languageRule + " תקן שגיאות כתיב ודקדוק בלבד בלי לשנות עובדות או משמעות. החזר רק את הטקסט המתוקן.", nil
	case "tags":
		return languageRule + " צור עד 12 תגיות חיפוש חזקות ורלוונטיות למוצר. אם הקלט כבר כולל תגיות, שמור את הטובות שבהן והרחב אותן במילים שאנשים באמת עשויים לחפש. כלול סוג מוצר, מותג/דגם רק אם קיימים, שימוש, חומר/מידה אם קיימים ומילים נרדפות טבעיות. אל תמציא מותגים או דגמים. החזר תגיות מופרדות בפסיקים בלבד.", nil
	case "category":
		return languageRule + " בחר את קטגוריית Marketplace המתאימה ביותר מתוך הרשימה שהמשתמש מספק. החזר רק את מזהה הקטגוריה, בלי הסבר.", nil
	case "extract_fields":
		return languageRule + " חלץ רק עובדות מפורשות מהטקסט לשדות שהמשתמש מספק. החזר JSON שטוח בלבד. אם שדה לא ידוע השאר אותו כמחרוזת ריקה. אל תנחש.", nil
	case "variants_light":
		return fmt.Sprintf("%s צור %d גרסאות קרובות למקור. שמור את המבנה והמסר, אבל החלף מילים וביטויים טבעיים ושנה מעט את סדר המשפטים. שמור כל עובדה ומספר. הפרד בין הגרסאות בשורה שמכילה רק ---.", base, count), nil
	case "variants":
		return fmt.Sprintf("%s צור %d גרסאות שונות באמת של הטקסט. אל תסתפק בהחלפת מילה אחת. בכל גרסה שנה פתיח ומבנה, תוך שמירה על כל העובדות. הפרד בין הגרסאות בשורה שמכילה רק ---.", base, count), nil
	case "post_variant":
		return base + " צור וריאציה אחת חדשה לפוסט. החזר בדיוק בפורמט הבא, בלי Markdown ובלי טקסט נוסף:\nTITLE: <כותרת בעברית>\nDESCRIPTION: <תיאור בעברית>", nil
	default:
		return "", fmt.Errorf("unknown AI mode")
	}
}

func samplingForMode(mode string) (float64, float64) {
	switch mode {
	case "conservative":
		return 0.35, 0.75
	case "balanced":
		return 0.65, 0.88
	default:
		return 0.9, 0.95
	}
}

func hasHebrewText(s string) bool {
	hebrew := 0
	letters := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '\u0590' && r <= '\u05FF') {
			letters++
		}
		if r >= '\u0590' && r <= '\u05FF' {
			hebrew++
		}
	}
	return hebrew >= 4 && (letters == 0 || float64(hebrew)/float64(letters) >= 0.35)
}

func parsePostVariant(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	i := strings.Index(s, "TITLE:")
	j := strings.Index(s, "DESCRIPTION:")
	if i < 0 || j < 0 || j <= i {
		return "", "", false
	}
	title := strings.TrimSpace(s[i+len("TITLE:") : j])
	desc := strings.TrimSpace(s[j+len("DESCRIPTION:"):])
	if title == "" || desc == "" {
		return "", "", false
	}
	return title, desc, true
}

func callLocalAI(settings Settings, req aiRequest) (string, error) {
	if !settings.LocalAIEnabled {
		return "", fmt.Errorf("AI מקומי כבוי")
	}
	if err := ensureLocalAI(settings); err != nil {
		return "", err
	}
	instruction, err := aiInstruction(settings, req)
	if err != nil {
		return "", err
	}
	model := effectiveTextModel(settings).Filename
	if settings.AIModelMode == "manual" && strings.TrimSpace(settings.LocalAIModel) != "" {
		model = strings.TrimSpace(settings.LocalAIModel)
	}
	if model == "" {
		model = defaultLocalAIModel
	}
	prompt := instruction + "\n\n" + strings.TrimSpace(req.Text)
	body, _ := json.Marshal(openAIChatRequest{
		Model: model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: "אתה קופירייטר מכירות בכיר למודעות Marketplace. כתוב עברית טבעית, חדה ומשכנעת, לא תרגום מילולי ולא ספאם. הבלט יתרונות אמיתיים בלבד, שמור עובדות ומספרים, ואל תמציא פרטים. אל תציג חשיבה או הסברים פנימיים; החזר רק את התוצאה המבוקשת."},
			{Role: "user", Content: prompt},
		},
		Temperature: func() float64 { t, _ := samplingForMode(normalizedCreativeMode(settings, req)); return t }(),
		TopP:        func() float64 { _, p := samplingForMode(normalizedCreativeMode(settings, req)); return p }(),
		MaxTokens:   1800,
		Stream:      false,
	})
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(localAIBaseURL(settings)+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Local AI HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out openAIChatResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.Error != nil && strings.TrimSpace(out.Error.Message) != "" {
		return "", fmt.Errorf("Local AI: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("Local AI returned no text")
	}
	text := cleanLocalAIOutput(out.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("Local AI returned empty text")
	}
	if settings.AIHebrewOnly && req.Mode != "tags" && !hasHebrewText(text) {
		return "", fmt.Errorf("ה-AI לא החזיר עברית תקינה; נסה שוב או בדוק שהמודל Qwen3-4B מותקן")
	}
	return text, nil
}

func generateCreativePost(settings Settings, title, description string) (string, string, error) {
	req := aiRequest{Mode: "post_variant", Title: title, Text: "כותרת מקור: " + strings.TrimSpace(title) + "\nתיאור מקור: " + strings.TrimSpace(description), Creativity: settings.AICreativeMode, Language: "he", Count: 1}
	out, _, err := callAI(settings, req)
	if err != nil {
		return "", "", err
	}
	t, d, ok := parsePostVariant(out)
	if !ok {
		return "", "", fmt.Errorf("AI returned an invalid post format")
	}
	if settings.AIHebrewOnly && (!hasHebrewText(t) || !hasHebrewText(d)) {
		return "", "", fmt.Errorf("AI post variation was not Hebrew")
	}
	return t, d, nil
}

func cleanLocalAIOutput(s string) string {
	s = strings.TrimSpace(s)
	for {
		a := strings.Index(strings.ToLower(s), "<think>")
		b := strings.Index(strings.ToLower(s), "</think>")
		if a < 0 || b < a {
			break
		}
		s = strings.TrimSpace(s[:a] + s[b+len("</think>"):])
	}
	return strings.TrimSpace(s)
}

func callAI(settings Settings, req aiRequest) (string, string, error) {
	if settings.LocalAIEnabled {
		if text, err := callLocalAI(settings, req); err == nil {
			return text, "local", nil
		} else if !settings.GeminiEnabled {
			return "", "local", err
		}
	}
	if settings.GeminiEnabled {
		text, err := callGemini(settings, req)
		if err != nil {
			return "", "gemini", err
		}
		return text, "gemini", nil
	}
	return "", "", fmt.Errorf("לא הוגדר מנוע AI. התקן/הפעל AI מקומי או הגדר Gemini")
}
