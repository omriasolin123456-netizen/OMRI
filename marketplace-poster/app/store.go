package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu      sync.Mutex
	baseDir string
	file    string
	data    AppData
}

func NewStore(baseDir string) (*Store, error) {
	for _, d := range []string{"items", "screenshots", "backups", "accounts"} {
		if err := os.MkdirAll(filepath.Join(baseDir, d), 0o755); err != nil {
			return nil, err
		}
	}
	s := &Store{baseDir: baseDir, file: filepath.Join(baseDir, "state.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.file)
	if os.IsNotExist(err) || len(b) == 0 {
		s.data = AppData{Settings: defaultSettings()}
		// One-time migration from MarketplacePoster V1 (ads.json).
		oldPath := filepath.Join(s.baseDir, "ads.json")
		if oldBytes, oldErr := os.ReadFile(oldPath); oldErr == nil && len(oldBytes) > 0 {
			var oldAds []Ad
			if json.Unmarshal(oldBytes, &oldAds) == nil {
				for i := range oldAds {
					if oldAds[i].VariantMode == "" {
						oldAds[i].VariantMode = "sequence"
					}
					if oldAds[i].ImageLimit == 0 {
						oldAds[i].ImageLimit = 10
					}
					if oldAds[i].Status == "" {
						oldAds[i].Status = "ready"
					}
				}
				s.data.Ads = oldAds
			}
		}
		s.ensureDefaultsLocked()
		return s.saveLocked()
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		// migrate old ads.json if this is a new v2 install and state is invalid/absent
		return fmt.Errorf("failed to read state.json: %w", err)
	}
	s.ensureDefaultsLocked()
	return nil
}

func (s *Store) ensureDefaultsLocked() {
	def := defaultSettings()
	if s.data.Settings.DailyLimit == 0 {
		s.data.Settings.DailyLimit = def.DailyLimit
	}
	if s.data.Settings.RunLimit == 0 {
		s.data.Settings.RunLimit = def.RunLimit
	}
	if s.data.Settings.MinDelay == 0 {
		s.data.Settings.MinDelay = def.MinDelay
	}
	if s.data.Settings.MaxDelay == 0 {
		s.data.Settings.MaxDelay = def.MaxDelay
	}
	if s.data.Settings.MaxErrors == 0 {
		s.data.Settings.MaxErrors = def.MaxErrors
	}
	if s.data.Settings.DecisionTimeout == 0 {
		s.data.Settings.DecisionTimeout = def.DecisionTimeout
	}
	if s.data.Settings.DefaultCondition == "" {
		s.data.Settings.DefaultCondition = def.DefaultCondition
	}
	if s.data.Settings.DefaultImageLimit == 0 {
		s.data.Settings.DefaultImageLimit = def.DefaultImageLimit
	}
	if s.data.Settings.VariantMode == "" {
		s.data.Settings.VariantMode = def.VariantMode
	}
	if s.data.Settings.LocalAIBaseURL == "" {
		s.data.Settings.LocalAIBaseURL = def.LocalAIBaseURL
		s.data.Settings.LocalAIEnabled = true
		s.data.Settings.LocalAIAutoStart = true
	}
	if s.data.Settings.LocalAIModel == "" {
		s.data.Settings.LocalAIModel = def.LocalAIModel
	}
	if s.data.Settings.AIModelMode == "" {
		s.data.Settings.AIModelMode = def.AIModelMode
	}
	if s.data.Settings.AICreativeMode == "" {
		s.data.Settings.AICreativeMode = def.AICreativeMode
		s.data.Settings.AIHebrewOnly = def.AIHebrewOnly
		s.data.Settings.AutoCreativeText = def.AutoCreativeText
		s.data.Settings.ImageShuffle = def.ImageShuffle
		s.data.Settings.ImageRandomCover = def.ImageRandomCover
		s.data.Settings.ImageRandomSubset = def.ImageRandomSubset
	}
	if s.data.Settings.AIVariantCount == 0 {
		s.data.Settings.AIVariantCount = def.AIVariantCount
	}
	if s.data.Settings.ImageMinCount == 0 {
		s.data.Settings.ImageMinCount = def.ImageMinCount
	}
	if s.data.Settings.GeminiModel == "" {
		s.data.Settings.GeminiModel = def.GeminiModel
	}
	if s.data.Settings.UpdateManifestURL == "" {
		s.data.Settings.UpdateManifestURL = def.UpdateManifestURL
	}
	if len(s.data.Accounts) == 0 {
		id := "default"
		s.data.Accounts = []Account{{ID: id, Name: "חשבון ראשי", CreatedAt: nowRFC3339()}}
		s.data.Settings.ActiveAccountID = id
	}
	if s.data.Settings.ActiveAccountID == "" {
		s.data.Settings.ActiveAccountID = s.data.Accounts[0].ID
	}
	if s.data.Ads == nil {
		s.data.Ads = []Ad{}
	}
	for i := range s.data.Ads {
		if s.data.Ads[i].Fields == nil {
			s.data.Ads[i].Fields = map[string]string{}
		}
		if s.data.Ads[i].CategoryID == "" && s.data.Ads[i].Category != "" {
			c := inferCategory(s.data.Ads[i].Category + " " + s.data.Ads[i].Title)
			s.data.Ads[i].CategoryID = c.ID
		}
	}
	if s.data.History == nil {
		s.data.History = []HistoryEntry{}
	}
	if s.data.Templates == nil {
		s.data.Templates = []Template{}
	}
}

func (s *Store) saveLocked() error {
	s.data.SavedAt = nowRFC3339()
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.file); err != nil {
		return err
	}
	return nil
}

func (s *Store) List(includeArchived bool) []Ad {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Ad, 0, len(s.data.Ads))
	for _, ad := range s.data.Ads {
		if !includeArchived && ad.Archived {
			continue
		}
		out = append(out, ad)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) Get(id string) (Ad, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ad := range s.data.Ads {
		if ad.ID == id {
			return ad, true
		}
	}
	return Ad{}, false
}

func (s *Store) Add(ad Ad, force bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !force {
		if dup := s.findDuplicateLocked(ad); dup != "" {
			return dup, fmt.Errorf("duplicate")
		}
	}
	s.data.Ads = append(s.data.Ads, ad)
	return "", s.saveLocked()
}

func (s *Store) UpdateAd(ad Ad) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Ads {
		if s.data.Ads[i].ID == ad.ID {
			ad.CreatedAt = s.data.Ads[i].CreatedAt
			ad.UpdatedAt = nowRFC3339()
			s.data.Ads[i] = ad
			return s.saveLocked()
		}
	}
	return fmt.Errorf("ad not found")
}

func (s *Store) PatchAd(id string, patch map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Ads {
		if s.data.Ads[i].ID != id {
			continue
		}
		ad := &s.data.Ads[i]
		if v, ok := patch["title"].(string); ok {
			ad.Title = strings.TrimSpace(v)
		}
		if v, ok := patch["price"].(string); ok {
			ad.Price = strings.TrimSpace(v)
		}
		if v, ok := patch["category_id"].(string); ok {
			ad.CategoryID = strings.TrimSpace(v)
		}
		if v, ok := patch["category"].(string); ok {
			ad.Category = strings.TrimSpace(v)
		}
		if m, ok := patch["fields"].(map[string]any); ok {
			ad.Fields = map[string]string{}
			for k, raw := range m {
				if v, ok := raw.(string); ok {
					ad.Fields[k] = strings.TrimSpace(v)
				}
			}
		}
		if v, ok := patch["condition"].(string); ok {
			ad.Condition = strings.TrimSpace(v)
		}
		if v, ok := patch["description"].(string); ok {
			ad.Description = v
		}
		if v, ok := patch["variant_mode"].(string); ok {
			ad.VariantMode = v
		}
		if v, ok := patch["archived"].(bool); ok {
			ad.Archived = v
		}
		if v, ok := patch["account_id"].(string); ok {
			ad.AccountID = v
		}
		if v, ok := patch["image_limit"].(float64); ok {
			ad.ImageLimit = int(v)
		}
		if v, ok := patch["primary_image"].(float64); ok {
			ad.PrimaryImage = int(v)
		}
		if arr, ok := patch["cover_blocked"].([]any); ok {
			ad.CoverBlocked = []int{}
			seen := map[int]bool{}
			for _, raw := range arr {
				if v, ok := raw.(float64); ok {
					i := int(v)
					if i >= 0 && i < len(ad.Images) && !seen[i] {
						seen[i] = true
						ad.CoverBlocked = append(ad.CoverBlocked, i)
					}
				}
			}
		}
		if v, ok := patch["use_signature"].(bool); ok {
			ad.UseSignature = &v
		}
		if v, ok := patch["signature_text"].(string); ok {
			ad.SignatureText = v
		}
		if arr, ok := patch["tags"].([]any); ok {
			ad.Tags = anyStrings(arr)
		}
		if arr, ok := patch["groups"].([]any); ok {
			ad.Groups = anyStrings(arr)
		}
		if arr, ok := patch["title_variants"].([]any); ok {
			ad.TitleVariants = anyStrings(arr)
		}
		if arr, ok := patch["description_variants"].([]any); ok {
			ad.DescriptionVariants = anyStrings(arr)
		}
		ad.UpdatedAt = nowRFC3339()
		return s.saveLocked()
	}
	return fmt.Errorf("ad not found")
}

func anyStrings(in []any) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if x, ok := v.(string); ok && strings.TrimSpace(x) != "" {
			out = append(out, strings.TrimSpace(x))
		}
	}
	return out
}

func (s *Store) Duplicate(id string) (Ad, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, src := range s.data.Ads {
		if src.ID != id {
			continue
		}
		ad := src
		ad.ID = newID()
		ad.Title += " - עותק"
		ad.Status, ad.Error, ad.PublishedURL, ad.PublishedAt = "ready", "", "", ""
		ad.PublishCount = 0
		ad.CreatedAt, ad.UpdatedAt = nowRFC3339(), ""
		newDir := filepath.Join(s.baseDir, "items", ad.ID)
		_ = os.MkdirAll(newDir, 0o755)
		newImages := []string{}
		for n, p := range src.Images {
			ext := filepath.Ext(p)
			dst := filepath.Join(newDir, fmt.Sprintf("%02d%s", n+1, ext))
			if copyFile(p, dst) == nil {
				newImages = append(newImages, dst)
			}
		}
		ad.Images = newImages
		s.data.Ads = append(s.data.Ads, ad)
		return ad, s.saveLocked()
	}
	return Ad{}, fmt.Errorf("ad not found")
}

func (s *Store) UpdateStatus(id, status, errText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Ads {
		if s.data.Ads[i].ID == id {
			s.data.Ads[i].Status = status
			s.data.Ads[i].Error = errText
			s.data.Ads[i].UpdatedAt = nowRFC3339()
			return s.saveLocked()
		}
	}
	return fmt.Errorf("ad not found")
}

func (s *Store) MarkPublished(id, titleUsed, descriptionUsed, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Ads {
		if s.data.Ads[i].ID == id {
			a := &s.data.Ads[i]
			a.Status, a.Error = "published", ""
			a.PublishCount++
			a.LastTitleUsed, a.LastDescriptionUsed = titleUsed, descriptionUsed
			a.PublishedURL, a.PublishedAt, a.UpdatedAt = url, nowRFC3339(), nowRFC3339()
			return s.saveLocked()
		}
	}
	return fmt.Errorf("ad not found")
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.data.Ads {
		if s.data.Ads[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("ad not found")
	}
	s.data.Ads = append(s.data.Ads[:idx], s.data.Ads[idx+1:]...)
	_ = os.RemoveAll(filepath.Join(s.baseDir, "items", id))
	return s.saveLocked()
}

func (s *Store) ItemDir(id string) string { return filepath.Join(s.baseDir, "items", id) }
func (s *Store) ScreenshotDir() string    { return filepath.Join(s.baseDir, "screenshots") }
func (s *Store) AccountProfileDir(id string) string {
	modern := filepath.Join(s.baseDir, "accounts", id, "browser_profile")
	// Keep the V1 Facebook session for the default account when it exists.
	if id == "default" {
		legacy := filepath.Join(s.baseDir, "browser_profile")
		if _, err := os.Stat(legacy); err == nil {
			if _, err2 := os.Stat(modern); os.IsNotExist(err2) {
				return legacy
			}
		}
	}
	return modern
}

func (s *Store) AddImages(id string, paths []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Ads {
		if s.data.Ads[i].ID == id {
			s.data.Ads[i].Images = append(s.data.Ads[i].Images, paths...)
			s.data.Ads[i].UpdatedAt = nowRFC3339()
			if s.data.Ads[i].Status == "needs_images" && len(s.data.Ads[i].Images) > 0 {
				s.data.Ads[i].Status = "ready"
			}
			return s.saveLocked()
		}
	}
	return fmt.Errorf("ad not found")
}

func (s *Store) DeleteImage(id string, index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Ads {
		if s.data.Ads[i].ID != id {
			continue
		}
		if index < 0 || index >= len(s.data.Ads[i].Images) {
			return fmt.Errorf("image not found")
		}
		_ = os.Remove(s.data.Ads[i].Images[index])
		s.data.Ads[i].Images = append(s.data.Ads[i].Images[:index], s.data.Ads[i].Images[index+1:]...)
		blocked := make([]int, 0, len(s.data.Ads[i].CoverBlocked))
		for _, b := range s.data.Ads[i].CoverBlocked {
			if b == index {
				continue
			}
			if b > index {
				b--
			}
			blocked = append(blocked, b)
		}
		s.data.Ads[i].CoverBlocked = blocked
		if s.data.Ads[i].PrimaryImage == index {
			s.data.Ads[i].PrimaryImage = 0
		} else if s.data.Ads[i].PrimaryImage > index {
			s.data.Ads[i].PrimaryImage--
		}
		if s.data.Ads[i].PrimaryImage >= len(s.data.Ads[i].Images) {
			s.data.Ads[i].PrimaryImage = 0
		}
		if len(s.data.Ads[i].Images) == 0 {
			s.data.Ads[i].Status = "needs_images"
		}
		return s.saveLocked()
	}
	return fmt.Errorf("ad not found")
}

func (s *Store) findDuplicateLocked(ad Ad) string {
	sig := adSignature(ad)
	for _, x := range s.data.Ads {
		if x.ID != ad.ID && adSignature(x) == sig {
			return x.ID
		}
	}
	return ""
}

func adSignature(ad Ad) string {
	h := sha256.New()
	_, _ = io.WriteString(h, strings.ToLower(strings.TrimSpace(ad.Title))+"|"+strings.TrimSpace(ad.Price)+"|")
	for _, p := range ad.Images {
		b, err := os.ReadFile(p)
		if err == nil {
			sum := sha256.Sum256(b)
			_, _ = h.Write(sum[:])
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Store) History(limit int) []HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]HistoryEntry, len(s.data.History))
	copy(out, s.data.History)
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) AddHistory(h HistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.History = append(s.data.History, h)
	if len(s.data.History) > 5000 {
		s.data.History = s.data.History[len(s.data.History)-5000:]
	}
	return s.saveLocked()
}

func (s *Store) Stats() DashboardStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := DashboardStats{}
	for _, a := range s.data.Ads {
		if a.Archived {
			st.Archived++
			continue
		}
		st.TotalAds++
		if a.Status == "ready" || a.Status == "needs_images" || a.Status == "error" {
			st.Waiting++
		}
	}
	today := time.Now().Format("2006-01-02")
	var durations int64
	var durationCount int64
	for _, h := range s.data.History {
		if len(h.StartedAt) >= 10 && h.StartedAt[:10] == today {
			switch h.Status {
			case "published":
				st.PublishedToday++
			case "error":
				st.FailedToday++
			case "skipped":
				st.SkippedToday++
			}
		}
		if h.DurationSeconds > 0 {
			durations += h.DurationSeconds
			durationCount++
		}
	}
	if durationCount > 0 {
		st.AvgSeconds = float64(durations) / float64(durationCount)
	}
	return st
}

func (s *Store) PublishedToday() int { return s.Stats().PublishedToday }

func (s *Store) Templates() []Template {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Template, len(s.data.Templates))
	copy(out, s.data.Templates)
	return out
}
func (s *Store) SaveTemplate(t *Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t == nil {
		return fmt.Errorf("template is required")
	}
	if t.ID == "" {
		t.ID = newID()
		t.CreatedAt = nowRFC3339()
		s.data.Templates = append(s.data.Templates, *t)
		return s.saveLocked()
	}
	for i := range s.data.Templates {
		if s.data.Templates[i].ID == t.ID {
			s.data.Templates[i] = *t
			return s.saveLocked()
		}
	}
	return fmt.Errorf("template not found")
}
func (s *Store) DeleteTemplate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Templates {
		if s.data.Templates[i].ID == id {
			s.data.Templates = append(s.data.Templates[:i], s.data.Templates[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("template not found")
}

func (s *Store) Accounts() []Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Account, len(s.data.Accounts))
	copy(out, s.data.Accounts)
	return out
}
func (s *Store) AddAccount(name string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := Account{ID: newID(), Name: strings.TrimSpace(name), CreatedAt: nowRFC3339()}
	if a.Name == "" {
		return Account{}, fmt.Errorf("name is required")
	}
	s.data.Accounts = append(s.data.Accounts, a)
	return a, s.saveLocked()
}
func (s *Store) DeleteAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.data.Accounts) <= 1 {
		return fmt.Errorf("cannot delete the last account")
	}
	for i, a := range s.data.Accounts {
		if a.ID == id {
			s.data.Accounts = append(s.data.Accounts[:i], s.data.Accounts[i+1:]...)
			if s.data.Settings.ActiveAccountID == id {
				s.data.Settings.ActiveAccountID = s.data.Accounts[0].ID
			}
			_ = os.RemoveAll(filepath.Join(s.baseDir, "accounts", id))
			return s.saveLocked()
		}
	}
	return fmt.Errorf("account not found")
}
func (s *Store) SetActiveAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.data.Accounts {
		if a.ID == id {
			s.data.Settings.ActiveAccountID = id
			return s.saveLocked()
		}
	}
	return fmt.Errorf("account not found")
}

func (s *Store) Settings() Settings { s.mu.Lock(); defer s.mu.Unlock(); return s.data.Settings }
func (s *Store) SaveSettings(v Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.DailyLimit < 1 {
		v.DailyLimit = 15
	}
	if v.RunLimit < 1 {
		v.RunLimit = 5
	}
	if v.MaxDelay < v.MinDelay {
		v.MaxDelay = v.MinDelay
	}
	if v.DecisionTimeout < 10 {
		v.DecisionTimeout = 60
	}
	if v.DefaultImageLimit < 1 || v.DefaultImageLimit > 10 {
		v.DefaultImageLimit = 10
	}
	if v.AICreativeMode != "conservative" && v.AICreativeMode != "balanced" && v.AICreativeMode != "creative" {
		v.AICreativeMode = "creative"
	}
	if v.AIVariantCount < 2 || v.AIVariantCount > 10 {
		v.AIVariantCount = 5
	}
	if v.ImageMinCount < 1 {
		v.ImageMinCount = 1
	}
	if v.ImageMinCount > v.DefaultImageLimit {
		v.ImageMinCount = v.DefaultImageLimit
	}
	if v.MaxGroups < 0 {
		v.MaxGroups = 0
	}
	s.data.Settings = v
	return s.saveLocked()
}

func (s *Store) Checkpoint() QueueCheckpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Checkpoint
}
func (s *Store) SaveCheckpoint(c QueueCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Checkpoint = c
	return s.saveLocked()
}
func (s *Store) ClearCheckpoint() error { return s.SaveCheckpoint(QueueCheckpoint{}) }

func (s *Store) BulkPatch(ids []string, patch map[string]any) error {
	for _, id := range ids {
		if err := s.PatchAd(id, patch); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ExportCSV(w io.Writer) error {
	ads := s.List(true)
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"id", "title", "price", "category", "condition", "description", "tags", "title_variants", "description_variants", "variant_mode", "groups", "account_id", "status", "published_url", "image_paths"})
	for _, a := range ads {
		_ = cw.Write([]string{a.ID, a.Title, a.Price, a.Category, a.Condition, a.Description, strings.Join(a.Tags, "|"), strings.Join(a.TitleVariants, "|"), strings.Join(a.DescriptionVariants, "|"), a.VariantMode, strings.Join(a.Groups, "|"), a.AccountID, a.Status, a.PublishedURL, strings.Join(a.Images, "|")})
	}
	return cw.Error()
}

func (s *Store) ImportCSV(r io.Reader) (int, error) {
	cr := csv.NewReader(r)
	rows, err := cr.ReadAll()
	if err != nil {
		return 0, err
	}
	if len(rows) < 2 {
		return 0, nil
	}
	head := map[string]int{}
	for i, k := range rows[0] {
		head[strings.ToLower(strings.TrimSpace(k))] = i
	}
	get := func(row []string, key string) string {
		if i, ok := head[key]; ok && i < len(row) {
			return row[i]
		}
		return ""
	}
	count := 0
	for _, row := range rows[1:] {
		a := Ad{ID: newID(), Title: get(row, "title"), Price: get(row, "price"), Category: get(row, "category"), Condition: get(row, "condition"), Description: get(row, "description"), Tags: splitPipe(get(row, "tags")), TitleVariants: splitPipe(get(row, "title_variants")), DescriptionVariants: splitPipe(get(row, "description_variants")), VariantMode: get(row, "variant_mode"), Groups: splitPipe(get(row, "groups")), AccountID: get(row, "account_id"), Status: "needs_images", CreatedAt: nowRFC3339()}
		if a.Title == "" {
			continue
		}
		if a.VariantMode == "" {
			a.VariantMode = "sequence"
		}
		paths := splitPipe(get(row, "image_paths"))
		dir := s.ItemDir(a.ID)
		_ = os.MkdirAll(dir, 0o755)
		for i, p := range paths {
			if _, err := os.Stat(p); err == nil {
				dst := filepath.Join(dir, fmt.Sprintf("%02d%s", i+1, filepath.Ext(p)))
				if copyFile(p, dst) == nil {
					a.Images = append(a.Images, dst)
				}
			}
		}
		if len(a.Images) > 0 {
			a.Status = "ready"
		}
		if _, err := s.Add(a, true); err == nil {
			count++
		}
	}
	return count, nil
}

func splitPipe(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	p := strings.Split(s, "|")
	out := []string{}
	for _, x := range p {
		if strings.TrimSpace(x) != "" {
			out = append(out, strings.TrimSpace(x))
		}
	}
	return out
}

func (s *Store) CreateBackup() (string, error) {
	name := "MarketplacePoster-backup-" + time.Now().Format("20060102-150405") + ".zip"
	path := filepath.Join(s.baseDir, "backups", name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(f)
	var add func(src, rel string) error
	add = func(src, rel string) error {
		info, err := os.Stat(src)
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				r, _ := filepath.Rel(src, p)
				return addFileToZip(zw, p, filepath.ToSlash(filepath.Join(rel, r)))
			})
		}
		return addFileToZip(zw, src, rel)
	}
	if err := add(s.file, "state.json"); err != nil {
		_ = zw.Close()
		_ = f.Close()
		return "", err
	}
	if err := add(filepath.Join(s.baseDir, "items"), "items"); err != nil {
		_ = zw.Close()
		_ = f.Close()
		return "", err
	}
	if err := zw.Close(); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func addFileToZip(zw *zip.Writer, src, name string) error {
	rf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer rf.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, rf)
	return err
}

func (s *Store) RestoreBackup(zipPath string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	tmp := filepath.Join(s.baseDir, "restore_tmp")
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		name := filepath.Clean(f.Name)
		if strings.Contains(name, "..") || filepath.IsAbs(name) {
			continue
		}
		dst := filepath.Join(tmp, name)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(dst, 0o755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(dst), 0o755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	state := filepath.Join(tmp, "state.json")
	if _, err := os.Stat(state); err != nil {
		return fmt.Errorf("backup has no state.json")
	}
	_ = os.RemoveAll(filepath.Join(s.baseDir, "items"))
	_ = os.MkdirAll(filepath.Join(s.baseDir, "items"), 0o755)
	if err := copyTree(filepath.Join(tmp, "items"), filepath.Join(s.baseDir, "items")); err != nil {
		return err
	}
	if err := copyFile(state, s.file); err != nil {
		return err
	}
	return s.load()
}

func copyTree(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func newID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }
func parseTags(raw string) []string {
	parts := strings.Split(raw, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
func parseLines(raw string) []string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n---\n")
	out := []string{}
	for _, x := range lines {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
func intString(v int) string { return strconv.Itoa(v) }
