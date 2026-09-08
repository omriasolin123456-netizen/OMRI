package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//go:embed web/*
var embeddedWeb embed.FS

type App struct {
	store     *Store
	hub       *BrowserHub
	publisher *Publisher
	license   *LicenseManager
	dataDir   string
	authToken string
}

func main() {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	dataDir := filepath.Join(base, "MarketplacePoster")
	_ = os.MkdirAll(dataDir, 0o755)
	logFile, _ := os.OpenFile(filepath.Join(dataDir, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if logFile != nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}
	store, err := NewStore(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	hub := NewBrowserHub(store)
	defer hub.CloseAll()
	license := NewLicenseManager(dataDir)
	app := &App{store: store, hub: hub, license: license, dataDir: dataDir, authToken: randomToken()}
	app.publisher = NewPublisher(store, hub, license)
	mux := http.NewServeMux()
	app.routes(mux)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + ln.Addr().String()
	go func() { time.Sleep(350 * time.Millisecond); _ = openBrowser(url) }()
	log.Printf("MarketplacePoster %s started at %s", AppVersion, url)
	if err := http.Serve(ln, mux); err != nil {
		log.Fatal(err)
	}
}

func (a *App) routes(mux *http.ServeMux) {
	webRoot, _ := fs.Sub(embeddedWeb, "web")
	mux.Handle("/", http.FileServer(http.FS(webRoot)))
	mux.HandleFunc("/api/auth/status", a.handleAuthStatus)
	mux.HandleFunc("/api/auth/unlock", a.handleAuthUnlock)
	mux.HandleFunc("/api/license/status", a.handleLicenseStatus)
	mux.HandleFunc("/api/license/activate", a.handleLicenseActivate)
	mux.HandleFunc("/api/license/trial", a.handleLicenseTrial)
	mux.HandleFunc("/api/license/deactivate", a.handleLicenseDeactivate)

	api := http.NewServeMux()
	api.HandleFunc("/api/ads", a.handleAds)
	api.HandleFunc("/api/ads/", a.handleAd)
	api.HandleFunc("/api/connect", a.handleConnect)
	api.HandleFunc("/api/state", a.handleState)
	api.HandleFunc("/api/publish", a.handlePublish)
	api.HandleFunc("/api/stop", a.handleStop)
	api.HandleFunc("/api/resume", a.handleResume)
	api.HandleFunc("/api/decision", a.handleDecision)
	api.HandleFunc("/api/history", a.handleHistory)
	api.HandleFunc("/api/settings", a.handleSettings)
	api.HandleFunc("/api/templates", a.handleTemplates)
	api.HandleFunc("/api/templates/", a.handleTemplate)
	api.HandleFunc("/api/accounts", a.handleAccounts)
	api.HandleFunc("/api/accounts/", a.handleAccount)
	api.HandleFunc("/api/bulk", a.handleBulk)
	api.HandleFunc("/api/csv/export", a.handleCSVExport)
	api.HandleFunc("/api/csv/import", a.handleCSVImport)
	api.HandleFunc("/api/folder/import", a.handleFolderImport)
	api.HandleFunc("/api/backup/create", a.handleBackupCreate)
	api.HandleFunc("/api/backup/download", a.handleBackupDownload)
	api.HandleFunc("/api/backup/restore", a.handleBackupRestore)
	api.HandleFunc("/api/screenshots/", a.handleScreenshot)
	api.HandleFunc("/api/ai", a.handleAI)
	api.HandleFunc("/api/ai/status", a.handleAIStatus)
	api.HandleFunc("/api/ai/start", a.handleAIStart)
	api.HandleFunc("/api/ai/install", a.handleAIInstall)
	api.HandleFunc("/api/ai/repair", a.handleAIRepair)
	api.HandleFunc("/api/ai/hardware", a.handleAIHardware)
	api.HandleFunc("/api/ai/models/install", a.handleAIModelInstall)
	api.HandleFunc("/api/ai/models/install-all", a.handleAIModelsInstallAll)
	api.HandleFunc("/api/ai/models/select", a.handleAIModelSelect)
	api.HandleFunc("/api/ai/image/status", a.handleImageAIStatus)
	api.HandleFunc("/api/ai/image/install", a.handleImageAIInstall)
	api.HandleFunc("/api/catalog/categories", a.handleCategories)
	api.HandleFunc("/api/open-data", a.handleOpenData)
	api.HandleFunc("/api/update/check", a.handleUpdateCheck)
	api.HandleFunc("/api/update/download", a.handleUpdateDownload)
	api.HandleFunc("/api/update/install", a.handleUpdateInstall)
	api.HandleFunc("/api/auth/pin", a.handleAuthPIN)
	api.HandleFunc("/api/exit", a.handleExit)
	mux.Handle("/api/", a.authMiddleware(a.licenseMiddleware(api)))
}

func (a *App) handleAds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		include := r.URL.Query().Get("archived") == "1"
		writeJSON(w, http.StatusOK, a.store.List(include))
	case http.MethodPost:
		a.createAd(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) createAd(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 300<<20)
	if err := r.ParseMultipartForm(300 << 20); err != nil {
		http.Error(w, "invalid upload: "+err.Error(), 400)
		return
	}
	settings := a.store.Settings()
	categoryID := strings.TrimSpace(r.FormValue("category_id"))
	category := strings.TrimSpace(r.FormValue("category"))
	if categoryID == "" && category != "" {
		categoryID = inferCategory(category).ID
	}
	if categoryID == "" && settings.DefaultCategory != "" {
		categoryID = inferCategory(settings.DefaultCategory).ID
	}
	if category == "" {
		category = categoryLabel(categoryID, settings.DefaultCategory)
	}
	fields := map[string]string{}
	if raw := strings.TrimSpace(r.FormValue("fields_json")); raw != "" {
		_ = json.Unmarshal([]byte(raw), &fields)
	}
	condition := strings.TrimSpace(r.FormValue("condition"))
	if condition == "" {
		condition = settings.DefaultCondition
	}
	useSig := r.FormValue("use_signature") == "1" || strings.EqualFold(r.FormValue("use_signature"), "true")
	ad := Ad{ID: newID(), Title: strings.TrimSpace(r.FormValue("title")), TitleVariants: parseLines(r.FormValue("title_variants")), Price: strings.TrimSpace(r.FormValue("price")), CategoryID: categoryID, Category: category, Fields: fields, Condition: condition, Description: strings.TrimSpace(r.FormValue("description")), DescriptionVariants: parseLines(r.FormValue("description_variants")), VariantMode: strings.TrimSpace(r.FormValue("variant_mode")), Tags: parseTags(r.FormValue("tags")), Groups: parseTags(r.FormValue("groups")), AccountID: strings.TrimSpace(r.FormValue("account_id")), ImageLimit: intForm(r, "image_limit", settings.DefaultImageLimit), PrimaryImage: intForm(r, "primary_image", 0), CoverBlocked: parseIntList(r.FormValue("cover_blocked")), UseSignature: &useSig, SignatureText: strings.TrimSpace(r.FormValue("signature_text")), Status: "ready", CreatedAt: nowRFC3339()}
	if ad.VariantMode == "" {
		ad.VariantMode = settings.VariantMode
	}
	if ad.AccountID == "" {
		ad.AccountID = settings.ActiveAccountID
	}
	if ad.Title == "" || ad.Price == "" || ad.Category == "" || ad.Condition == "" {
		http.Error(w, "title, price, category and condition are required", 400)
		return
	}
	files := r.MultipartForm.File["images"]
	if len(files) == 0 {
		http.Error(w, "at least one image is required", 400)
		return
	}
	if len(files) > 10 {
		http.Error(w, "maximum 10 images per ad", 400)
		return
	}
	dir := a.store.ItemDir(ad.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	for i, fh := range files {
		path, err := saveUploadedImage(dir, i+1, fh)
		if err != nil {
			_ = os.RemoveAll(dir)
			http.Error(w, err.Error(), 400)
			return
		}
		ad.Images = append(ad.Images, path)
	}
	force := r.FormValue("force") == "1" || r.FormValue("force") == "true"
	dup, err := a.store.Add(ad, force)
	if err != nil {
		_ = os.RemoveAll(dir)
		if err.Error() == "duplicate" {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "duplicate", "duplicate_id": dup})
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, http.StatusCreated, ad)
}

func saveUploadedImage(dir string, index int, fh *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".heic": true, ".heif": true}
	if !allowed[ext] {
		return "", fmt.Errorf("unsupported image type: %s", ext)
	}
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	name := fmt.Sprintf("%02d%s", index, ext)
	path := filepath.Join(dir, name)
	dst, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return path, err
}

func (a *App) handleAd(w http.ResponseWriter, r *http.Request) {
	rel := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/ads/"), "/")
	parts := strings.Split(rel, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) > 1 {
		switch parts[1] {
		case "image":
			a.handleAdImage(w, r, id, parts)
			return
		case "images":
			a.handleAdImages(w, r, id)
			return
		case "duplicate":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", 405)
				return
			}
			ad, err := a.store.Duplicate(id)
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			writeJSON(w, 201, ad)
			return
		case "variants":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", 405)
				return
			}
			if a.publisher.State().Running {
				http.Error(w, "cannot create variants while publishing", 409)
				return
			}
			src, ok := a.store.Get(id)
			if !ok {
				http.NotFound(w, r)
				return
			}
			var req VariantCreateRequest
			if json.NewDecoder(r.Body).Decode(&req) != nil {
				http.Error(w, "invalid json", 400)
				return
			}
			created, err := a.createVariantCopies(src, req.Count, req.UseAIImages)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			ids := make([]string, 0, len(created))
			for _, ad := range created {
				ids = append(ids, ad.ID)
			}
			if req.AutoPublish && len(ids) > 0 {
				set := a.store.Settings()
				err = a.publisher.Start(RunOptions{IDs: ids, Limit: len(ids), MinDelay: set.MinDelay, MaxDelay: set.MaxDelay, Groups: set.PostToGroups, PreviewBeforePublish: req.Preview || set.PreviewBeforePublish, AccountID: set.ActiveAccountID})
				if err != nil {
					http.Error(w, err.Error(), 409)
					return
				}
			}
			writeJSON(w, 201, map[string]any{"ads": created, "ids": ids, "publishing": req.AutoPublish})
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		ad, ok := a.store.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, 200, ad)
	case http.MethodPatch:
		if a.publisher.State().Running {
			http.Error(w, "cannot edit while publishing", 409)
			return
		}
		var patch map[string]any
		if json.NewDecoder(r.Body).Decode(&patch) != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		if err := a.store.PatchAd(id, patch); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		ad, _ := a.store.Get(id)
		writeJSON(w, 200, ad)
	case http.MethodDelete:
		if a.publisher.State().Running {
			http.Error(w, "cannot delete while publishing", 409)
			return
		}
		if err := a.store.Delete(id); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (a *App) handleAdImage(w http.ResponseWriter, r *http.Request, id string, parts []string) {
	ad, ok := a.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	idx := 0
	if len(parts) > 2 {
		idx, _ = strconv.Atoi(parts[2])
	}
	if idx < 0 || idx >= len(ad.Images) {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, ad.Images[idx])
		return
	}
	if r.Method == http.MethodDelete {
		if err := a.store.DeleteImage(id, idx); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.WriteHeader(204)
		return
	}
	http.Error(w, "method not allowed", 405)
}

func (a *App) handleAdImages(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ad, ok := a.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	files := r.MultipartForm.File["images"]
	if len(ad.Images)+len(files) > 10 {
		http.Error(w, "maximum 10 images per ad", 400)
		return
	}
	dir := a.store.ItemDir(id)
	paths := []string{}
	for i, fh := range files {
		p, err := saveUploadedImage(dir, len(ad.Images)+i+1, fh)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		paths = append(paths, p)
	}
	if err := a.store.AddImages(id, paths); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	updated, _ := a.store.Get(id)
	writeJSON(w, 200, updated)
}

func (a *App) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	s, id, err := a.hub.Active()
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	status, _ := s.Status()
	if status == "connecting" || status == "connected" {
		writeJSON(w, 202, map[string]string{"status": status, "account_id": id})
		return
	}
	go func() {
		if err := s.Connect(); err != nil {
			log.Printf("Facebook connect: %v", err)
		}
	}()
	writeJSON(w, 202, map[string]string{"status": "connecting", "account_id": id})
}

type publishRequest struct {
	IDs       []string `json:"ids"`
	Limit     int      `json:"limit"`
	MinDelay  int      `json:"min_delay"`
	MaxDelay  int      `json:"max_delay"`
	Groups    bool     `json:"groups"`
	Preview   bool     `json:"preview"`
	DryRun    bool     `json:"dry_run"`
	AccountID string   `json:"account_id"`
}

func (a *App) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	maxAccounts := a.license.MaxAccounts()
	if maxAccounts > 0 && len(a.store.Accounts()) > maxAccounts {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "facebook_account_limit_exceeded", "max_accounts": maxAccounts})
		return
	}
	var req publishRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if err := a.publisher.Start(RunOptions{IDs: req.IDs, Limit: req.Limit, MinDelay: req.MinDelay, MaxDelay: req.MaxDelay, Groups: req.Groups, PreviewBeforePublish: req.Preview, DryRun: req.DryRun, AccountID: req.AccountID}); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	writeJSON(w, 202, map[string]string{"status": "started"})
}
func (a *App) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	a.publisher.Stop()
	writeJSON(w, 202, map[string]string{"status": "stopping"})
}
func (a *App) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := a.publisher.Resume(); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	writeJSON(w, 202, map[string]string{"status": "resumed"})
}
func (a *App) handleDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		Action string `json:"action"`
	}
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if err := a.publisher.ResolveDecision(v.Action); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "accepted"})
}

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	settings := a.store.Settings()
	fb, fbErr := a.hub.Status(settings.ActiveAccountID)
	writeJSON(w, 200, map[string]any{"version": AppVersion, "facebook": fb, "facebook_error": fbErr, "active_account_id": settings.ActiveAccountID, "queue": a.publisher.State(), "checkpoint": a.store.Checkpoint(), "stats": a.store.Stats(), "data_dir": a.dataDir, "license": a.license.Status(len(a.store.Accounts()), false)})
}
func (a *App) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, 200, a.store.History(limit))
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		v := a.store.Settings()
		v.PINHash = ""
		writeJSON(w, 200, v)
	case http.MethodPut:
		var v Settings
		if json.NewDecoder(r.Body).Decode(&v) != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		old := a.store.Settings()
		if v.ActiveAccountID == "" {
			v.ActiveAccountID = old.ActiveAccountID
		}
		v.PINHash = old.PINHash
		if err := a.store.SaveSettings(v); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		v.PINHash = ""
		writeJSON(w, 200, v)
	default:
		http.Error(w, "method not allowed", 405)
	}
}
func (a *App) handleTemplates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, a.store.Templates())
	case http.MethodPost:
		var t Template
		if json.NewDecoder(r.Body).Decode(&t) != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		if t.ID == "" {
			t.CreatedAt = nowRFC3339()
		}
		if err := a.store.SaveTemplate(&t); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 201, t)
	default:
		http.Error(w, "method not allowed", 405)
	}
}
func (a *App) handleTemplate(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/templates/"), "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		if err := a.store.DeleteTemplate(id); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.WriteHeader(204)
		return
	}
	http.Error(w, "method not allowed", 405)
}

func (a *App) handleAccounts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, map[string]any{"accounts": a.store.Accounts(), "active": a.store.Settings().ActiveAccountID})
	case http.MethodPost:
		maxAccounts := a.license.MaxAccounts()
		if maxAccounts > 0 && len(a.store.Accounts()) >= maxAccounts {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "account_limit", "max_accounts": maxAccounts})
			return
		}
		var v struct {
			Name string `json:"name"`
		}
		if json.NewDecoder(r.Body).Decode(&v) != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		acc, err := a.store.AddAccount(v.Name)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, 201, acc)
	default:
		http.Error(w, "method not allowed", 405)
	}
}
func (a *App) handleAccount(w http.ResponseWriter, r *http.Request) {
	rel := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/accounts/"), "/")
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	switch {
	case r.Method == http.MethodPost && action == "activate":
		if a.publisher.State().Running {
			http.Error(w, "cannot switch account while publishing", 409)
			return
		}
		if err := a.store.SetActiveAccount(id); err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		writeJSON(w, 200, map[string]string{"active": id})
	case r.Method == http.MethodPost && action == "connect":
		s, err := a.hub.Session(id)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		go s.Connect()
		writeJSON(w, 202, map[string]string{"status": "connecting"})
	case r.Method == http.MethodDelete:
		if a.publisher.State().Running {
			http.Error(w, "cannot delete account while publishing", 409)
			return
		}
		a.hub.Close(id)
		if err := a.store.DeleteAccount(id); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (a *App) handleBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		IDs   []string       `json:"ids"`
		Patch map[string]any `json:"patch"`
	}
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	if err := a.store.BulkPatch(v.IDs, v.Patch); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 200, map[string]int{"updated": len(v.IDs)})
}
func (a *App) handleCSVExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=marketplace-ads.csv")
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	if err := a.store.ExportCSV(w); err != nil {
		log.Printf("csv export: %v", err)
	}
}
func (a *App) handleCSVImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "CSV file is required", 400)
		return
	}
	defer f.Close()
	n, err := a.store.ImportCSV(f)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 200, map[string]int{"imported": n})
}

func (a *App) handleFolderImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files", 400)
		return
	}
	type group struct {
		meta   map[string]string
		images []*multipart.FileHeader
	}
	groups := map[string]*group{}
	paths := r.MultipartForm.Value["paths"]
	for fileIndex, fh := range files {
		name := filepath.ToSlash(fh.Filename)
		if fileIndex < len(paths) && strings.TrimSpace(paths[fileIndex]) != "" {
			name = filepath.ToSlash(paths[fileIndex])
		}
		parts := strings.Split(name, "/")
		folder := "root"
		if len(parts) > 1 {
			folder = parts[0]
		}
		g := groups[folder]
		if g == nil {
			g = &group{meta: map[string]string{}}
			groups[folder] = g
		}
		if strings.EqualFold(filepath.Base(name), "description.txt") {
			f, _ := fh.Open()
			b, _ := io.ReadAll(io.LimitReader(f, 1<<20))
			_ = f.Close()
			g.meta = parseDescriptionText(string(b))
		} else {
			ext := strings.ToLower(filepath.Ext(name))
			if map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".heic": true, ".heif": true}[ext] {
				g.images = append(g.images, fh)
			}
		}
	}
	settings := a.store.Settings()
	created := 0
	for folder, g := range groups {
		if len(g.images) == 0 {
			continue
		}
		title := g.meta["title"]
		if title == "" {
			title = folder
		}
		price := g.meta["price"]
		if price == "" {
			price = "0"
		}
		category := g.meta["category"]
		if category == "" {
			category = settings.DefaultCategory
		}
		condition := g.meta["condition"]
		if condition == "" {
			condition = settings.DefaultCondition
		}
		if category == "" {
			continue
		}
		ad := Ad{ID: newID(), Title: title, Price: price, Category: category, Condition: condition, Description: g.meta["description"], Tags: parseTags(g.meta["tags"]), VariantMode: settings.VariantMode, AccountID: settings.ActiveAccountID, ImageLimit: settings.DefaultImageLimit, Status: "ready", CreatedAt: nowRFC3339()}
		dir := a.store.ItemDir(ad.ID)
		_ = os.MkdirAll(dir, 0o755)
		for i, fh := range g.images {
			if i >= 10 {
				break
			}
			p, err := saveUploadedImage(dir, i+1, fh)
			if err == nil {
				ad.Images = append(ad.Images, p)
			}
		}
		if len(ad.Images) > 0 {
			if _, err := a.store.Add(ad, true); err == nil {
				created++
			}
		}
	}
	writeJSON(w, 200, map[string]int{"created": created})
}

func (a *App) handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	path, err := a.store.CreateBackup()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, map[string]string{"name": filepath.Base(path), "download": "/api/backup/download?name=" + filepath.Base(path)})
}
func (a *App) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}
	path := filepath.Join(a.dataDir, "backups", name)
	http.ServeFile(w, r, path)
}
func (a *App) handleBackupRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if a.publisher.State().Running {
		http.Error(w, "stop publishing before restore", 409)
		return
	}
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "backup zip is required", 400)
		return
	}
	defer f.Close()
	tmp := filepath.Join(a.dataDir, "restore-upload.zip")
	out, err := os.Create(tmp)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_, err = io.Copy(out, f)
	_ = out.Close()
	if err == nil {
		err = a.store.RestoreBackup(tmp)
	}
	_ = os.Remove(tmp)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "restored"})
}
func (a *App) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/api/screenshots/"))
	if name == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(a.store.ScreenshotDir(), name))
}
func (a *App) handleAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req aiRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	text, engine, err := callAI(a.store.Settings(), req)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 200, map[string]string{"text": text, "engine": engine})
}

func (a *App) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	writeJSON(w, 200, localAIStatus(a.store.Settings()))
}

func (a *App) handleAIStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	settings := a.store.Settings()
	if err := ensureLocalAI(settings); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 200, localAIStatus(settings))
}
func (a *App) handleAIInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	settings := a.store.Settings()
	settings.LocalAIEnabled = true
	settings.LocalAIAutoStart = true
	settings.AIModelMode = "auto"
	if err := a.store.SaveSettings(settings); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := launchSmartAIInstall(settings); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	st := localAIStatus(settings)
	st.Installing = true
	st.Message = "Smart AI מתחיל התקנה ברקע — אפשר להמשיך לעבוד"
	writeJSON(w, 202, st)
}

func (a *App) handleAIRepair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	settings := a.store.Settings()
	settings.LocalAIEnabled = true
	settings.LocalAIAutoStart = true
	if err := a.store.SaveSettings(settings); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := launchSmartAIRepair(settings); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	st := localAIStatus(settings)
	st.Installing = true
	st.State = "installing"
	st.Message = "Repair AI התחיל ברקע. אפשר להמשיך לעבוד בתוכנה."
	writeJSON(w, 202, st)
}

func (a *App) handleCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, 200, marketplaceCategories)
		return
	}
	writeJSON(w, 200, map[string]any{"match": inferCategory(q), "categories": marketplaceCategories})
}

func (a *App) handleAIHardware(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	h := detectHardware()
	settings := a.store.Settings()
	writeJSON(w, 200, map[string]any{"hardware": h, "installed": installedModels(settings), "effective": effectiveTextModel(settings), "image_ai": imageAIStatus(settings)})
}

func (a *App) handleAIModelInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		ID   string `json:"id"`
		Auto bool   `json:"auto"`
	}
	_ = json.NewDecoder(r.Body).Decode(&v)
	var p AIModelProfile
	var ok bool
	if v.Auto || strings.TrimSpace(v.ID) == "" {
		p = recommendModel(detectHardware())
		ok = true
	} else {
		p, ok = profileByID(v.ID)
	}
	if !ok {
		http.Error(w, "unknown model", 400)
		return
	}
	if err := launchModelInstall(a.store.Settings(), p); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 202, map[string]any{"status": "started", "model": p})
}

func (a *App) handleAIModelsInstallAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := launchAllModelsInstall(a.store.Settings()); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 202, map[string]any{"status": "started", "models": aiModelProfiles})
}

func (a *App) handleAIModelSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		ID   string `json:"id"`
		Mode string `json:"mode"`
	}
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	s := a.store.Settings()
	if v.Mode == "auto" {
		s.AIModelMode = "auto"
		p := effectiveTextModel(s)
		s.LocalAIModel = p.Filename
	} else {
		p, ok := profileByID(v.ID)
		if !ok {
			http.Error(w, "unknown model", 400)
			return
		}
		s.AIModelMode = "manual"
		s.LocalAIModel = p.Filename
	}
	if err := a.store.SaveSettings(s); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, map[string]any{"settings": s, "effective": effectiveTextModel(s)})
}

func (a *App) handleImageAIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	writeJSON(w, 200, imageAIStatus(a.store.Settings()))
}

func (a *App) handleImageAIInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	settings, err := launchImageAIInstall(a.store.Settings())
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := a.store.SaveSettings(settings); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 202, imageAIStatus(settings))
}

func (a *App) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	u := strings.TrimSpace(a.store.Settings().UpdateManifestURL)
	if u == "" {
		http.Error(w, "update manifest URL is not configured", 400)
		return
	}
	m, err := fetchUpdateManifest(u)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, 200, map[string]any{"current": AppVersion, "latest": m.Version, "update_available": versionGreater(m.Version, AppVersion), "notes": m.Notes, "download_url": m.DownloadURL, "sha256": m.SHA256})
}

func (a *App) handleUpdateDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	u := strings.TrimSpace(a.store.Settings().UpdateManifestURL)
	if u == "" {
		http.Error(w, "update manifest URL is not configured", 400)
		return
	}
	m, err := fetchUpdateManifest(u)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if !versionGreater(m.Version, AppVersion) {
		http.Error(w, "no newer version is available", 409)
		return
	}
	path, err := downloadUpdate(a.dataDir, m)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = openPath(filepath.Dir(path))
	writeJSON(w, 200, map[string]string{"status": "downloaded", "path": path})
}

func (a *App) handleUpdateInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	u := strings.TrimSpace(a.store.Settings().UpdateManifestURL)
	if u == "" {
		http.Error(w, "update manifest URL is not configured", 400)
		return
	}
	m, err := fetchUpdateManifest(u)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if !versionGreater(m.Version, AppVersion) {
		http.Error(w, "no newer version is available", 409)
		return
	}
	path, err := downloadUpdate(a.dataDir, m)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := scheduleSelfUpdate(path); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "restarting", "version": m.Version})
	go func() {
		time.Sleep(1200 * time.Millisecond)
		os.Exit(0)
	}()
}

func (a *App) handleOpenData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := openPath(a.dataDir); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, map[string]string{"status": "opened"})
}

func (a *App) handleLicenseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	status := a.license.Status(len(a.store.Accounts()), true)
	writeJSON(w, 200, status)
}

func (a *App) handleLicenseActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		Key string `json:"key"`
	}
	if json.NewDecoder(r.Body).Decode(&v) != nil || strings.TrimSpace(v.Key) == "" {
		http.Error(w, "invalid license key", 400)
		return
	}
	status, err := a.license.Activate(v.Key, len(a.store.Accounts()))
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, status)
}

func (a *App) handleLicenseTrial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	status, err := a.license.StartTrial(len(a.store.Accounts()))
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, status)
}

func (a *App) handleLicenseDeactivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if a.publisher.State().Running {
		http.Error(w, "cannot deactivate while publishing", 409)
		return
	}
	if err := a.license.Deactivate(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, map[string]any{"status": "deactivated"})
}

func (a *App) licenseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.license.Allowed() {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": "license_required"})
	})
}

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.store.Settings().PINHash == "" {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie("mp_session")
		if err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(a.authToken)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "locked"})
	})
}

func (a *App) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	configured := a.store.Settings().PINHash != ""
	unlocked := !configured
	if configured {
		if c, err := r.Cookie("mp_session"); err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(a.authToken)) == 1 {
			unlocked = true
		}
	}
	writeJSON(w, 200, map[string]any{"configured": configured, "locked": !unlocked})
}

func (a *App) handleAuthUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		PIN string `json:"pin"`
	}
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	h := sha256.Sum256([]byte(v.PIN))
	got := hex.EncodeToString(h[:])
	expected := a.store.Settings().PINHash
	if expected == "" || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		writeJSON(w, 401, map[string]string{"error": "PIN שגוי"})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "mp_session", Value: a.authToken, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]string{"status": "unlocked"})
}

func (a *App) handleAuthPIN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var v struct {
		PIN   string `json:"pin"`
		Clear bool   `json:"clear"`
	}
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	s := a.store.Settings()
	if v.Clear {
		s.PINHash = ""
	} else {
		if len(v.PIN) < 4 {
			http.Error(w, "PIN must be at least 4 characters", 400)
			return
		}
		h := sha256.Sum256([]byte(v.PIN))
		s.PINHash = hex.EncodeToString(h[:])
	}
	if err := a.store.SaveSettings(s); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if !v.Clear {
		http.SetCookie(w, &http.Cookie{Name: "mp_session", Value: a.authToken, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	writeJSON(w, 200, map[string]string{"status": "saved"})
}

func (a *App) handleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	a.publisher.Stop()
	a.hub.CloseAll()
	stopLocalAI()
	writeJSON(w, 200, map[string]string{"status": "exiting"})
	go func() { time.Sleep(300 * time.Millisecond); os.Exit(0) }()
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return newID() + newID()
	}
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
func openPath(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
func intForm(r *http.Request, key string, def int) int {
	v, err := strconv.Atoi(r.FormValue(key))
	if err != nil {
		return def
	}
	return v
}

func parseIntList(raw string) []int {
	values := make([]int, 0)
	seen := make(map[int]struct{})
	for _, part := range strings.Split(raw, ",") {
		v, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || v < 0 {
			continue
		}
		if _, exists := seen[v]; exists {
			continue
		}
		seen[v] = struct{}{}
		values = append(values, v)
	}
	return values
}
