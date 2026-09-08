package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"marketplaceposter/automation"
)

type DecisionState struct {
	Type       string   `json:"type"` // error, preview
	AdID       string   `json:"ad_id"`
	Title      string   `json:"title"`
	Error      string   `json:"error,omitempty"`
	ExpiresAt  int64    `json:"expires_at,omitempty"`
	Screenshot string   `json:"screenshot,omitempty"`
	Actions    []string `json:"actions"`
}

type QueueState struct {
	Running   bool           `json:"running"`
	Stop      bool           `json:"stop"`
	CurrentID string         `json:"current_id,omitempty"`
	Current   string         `json:"current"`
	Completed int            `json:"completed"`
	Failed    int            `json:"failed"`
	Skipped   int            `json:"skipped"`
	Total     int            `json:"total"`
	Message   string         `json:"message"`
	Decision  *DecisionState `json:"decision,omitempty"`
}

type Publisher struct {
	mu         sync.Mutex
	store      *Store
	hub        *BrowserHub
	license    *LicenseManager
	state      QueueState
	decisionCh chan string
}

func NewPublisher(store *Store, hub *BrowserHub, license *LicenseManager) *Publisher {
	return &Publisher{store: store, hub: hub, license: license, decisionCh: make(chan string, 1)}
}

func (p *Publisher) State() QueueState {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := p.state
	if p.state.Decision != nil {
		d := *p.state.Decision
		s.Decision = &d
	}
	return s
}

func (p *Publisher) setState(fn func(*QueueState)) { p.mu.Lock(); fn(&p.state); p.mu.Unlock() }

func (p *Publisher) Stop() {
	p.setState(func(s *QueueState) {
		if s.Running {
			s.Stop = true
			s.Message = "עוצר אחרי הפעולה הנוכחית..."
		}
	})
}

func (p *Publisher) ResolveDecision(action string) error {
	p.mu.Lock()
	if !p.state.Running || p.state.Decision == nil {
		p.mu.Unlock()
		return fmt.Errorf("there is no pending decision")
	}
	p.mu.Unlock()
	select {
	case p.decisionCh <- action:
		return nil
	default:
		return fmt.Errorf("decision already submitted")
	}
}

type RunOptions struct {
	IDs                  []string
	Limit                int
	MinDelay             int
	MaxDelay             int
	Groups               bool
	PreviewBeforePublish bool
	DryRun               bool
	AccountID            string
}

func (p *Publisher) Start(opts RunOptions) error {
	p.mu.Lock()
	if p.state.Running {
		p.mu.Unlock()
		return fmt.Errorf("a publishing queue is already running")
	}
	p.mu.Unlock()
	if opts.Limit <= 0 || opts.Limit > len(opts.IDs) {
		opts.Limit = len(opts.IDs)
	}
	if len(opts.IDs) == 0 {
		return fmt.Errorf("no ads selected")
	}
	settings := p.store.Settings()
	if opts.Limit > settings.RunLimit && settings.RunLimit > 0 {
		opts.Limit = settings.RunLimit
	}
	opts.IDs = append([]string(nil), opts.IDs[:opts.Limit]...)
	if opts.MinDelay < 0 {
		opts.MinDelay = 0
	}
	if opts.MaxDelay < opts.MinDelay {
		opts.MaxDelay = opts.MinDelay
	}
	if opts.AccountID == "" {
		opts.AccountID = settings.ActiveAccountID
	}
	checkpoint := QueueCheckpoint{Active: true, IDs: opts.IDs, NextIndex: 0, MinDelay: opts.MinDelay, MaxDelay: opts.MaxDelay, Groups: opts.Groups, PreviewBeforePublish: opts.PreviewBeforePublish, DryRun: opts.DryRun, AccountID: opts.AccountID, StartedAt: nowRFC3339()}
	_ = p.store.SaveCheckpoint(checkpoint)
	p.setState(func(s *QueueState) { *s = QueueState{Running: true, Total: len(opts.IDs), Message: "מתחיל..."} })
	go p.run(opts, 0)
	return nil
}

func (p *Publisher) Resume() error {
	c := p.store.Checkpoint()
	if !c.Active || c.NextIndex >= len(c.IDs) {
		return fmt.Errorf("there is no interrupted queue to resume")
	}
	p.mu.Lock()
	if p.state.Running {
		p.mu.Unlock()
		return fmt.Errorf("a publishing queue is already running")
	}
	p.mu.Unlock()
	opts := RunOptions{IDs: c.IDs, Limit: len(c.IDs), MinDelay: c.MinDelay, MaxDelay: c.MaxDelay, Groups: c.Groups, PreviewBeforePublish: c.PreviewBeforePublish, DryRun: c.DryRun, AccountID: c.AccountID}
	p.setState(func(s *QueueState) {
		*s = QueueState{Running: true, Total: len(c.IDs), Completed: c.NextIndex, Message: "ממשיך מהריצה הקודמת..."}
	})
	go p.run(opts, c.NextIndex)
	return nil
}

func (p *Publisher) run(opts RunOptions, startIndex int) {
	finishedAll := false
	defer func() {
		p.setState(func(s *QueueState) {
			s.Running = false
			s.Current = ""
			s.CurrentID = ""
			s.Decision = nil
			if s.Stop {
				s.Message = "נעצר"
			} else if finishedAll {
				s.Message = "הפרסום הסתיים"
			}
		})
		if finishedAll {
			_ = p.store.ClearCheckpoint()
		}
	}()
	settings := p.store.Settings()
	totalErrors := 0
	for idx := startIndex; idx < len(opts.IDs); idx++ {
		if p.State().Stop {
			return
		}
		if settings.DailyLimit > 0 && p.store.PublishedToday() >= settings.DailyLimit {
			p.setState(func(s *QueueState) { s.Message = "הגעת למגבלת הפרסום היומית שהגדרת" })
			return
		}
		id := opts.IDs[idx]
		ad, ok := p.store.Get(id)
		if !ok {
			p.advanceCheckpoint(idx + 1)
			continue
		}
		if len(ad.Images) == 0 {
			_ = p.store.UpdateStatus(id, "needs_images", "אין תמונות")
			p.advanceCheckpoint(idx + 1)
			continue
		}
		p.setState(func(s *QueueState) {
			s.CurrentID = id
			s.Current = ad.Title
			s.Message = fmt.Sprintf("מכין מודעה %d מתוך %d", idx+1, len(opts.IDs))
		})
		titleUsed, descUsed := chooseAdText(ad)
		if settings.AutoCreativeText {
			p.setState(func(s *QueueState) { s.Message = "AI מכין וריאציה חדשה בעברית..." })
			if aiTitle, aiDesc, err := generateCreativePost(settings, titleUsed, descUsed); err == nil {
				titleUsed, descUsed = aiTitle, aiDesc
			}
		}
		descUsed = decorateDescription(ad, settings, descUsed)
		accountID := ad.AccountID
		if accountID == "" {
			accountID = opts.AccountID
		}
		if accountID == "" {
			accountID = settings.ActiveAccountID
		}
		result := p.processAd(ad, titleUsed, descUsed, accountID, opts.Groups, opts.PreviewBeforePublish, opts.DryRun, settings, &totalErrors)
		switch result {
		case "published":
			p.setState(func(s *QueueState) { s.Completed++ })
		case "skipped":
			p.setState(func(s *QueueState) { s.Skipped++ })
		case "preview":
			p.setState(func(s *QueueState) { s.Completed++ })
		case "manual", "stopped":
			return
		}
		p.advanceCheckpoint(idx + 1)
		if settings.MaxErrors > 0 && totalErrors >= settings.MaxErrors {
			p.setState(func(s *QueueState) {
				s.Message = "ההרצה נעצרה לאחר מספר השגיאות המקסימלי שהוגדר"
			})
			return
		}
		if idx == len(opts.IDs)-1 {
			continue
		}
		delay := opts.MinDelay
		if opts.MaxDelay > opts.MinDelay {
			delay += rand.Intn(opts.MaxDelay - opts.MinDelay + 1)
		}
		for i := 0; i < delay; i++ {
			if p.State().Stop {
				return
			}
			p.setState(func(s *QueueState) {
				s.Message = fmt.Sprintf("ממתין %d שניות למודעה הבאה", delay-i)
			})
			time.Sleep(time.Second)
		}
	}
	finishedAll = true
}

func (p *Publisher) processAd(ad Ad, titleUsed, descUsed, accountID string, groups, preview, dryRun bool, settings Settings, totalErrors *int) string {
	for {
		if p.State().Stop {
			return "stopped"
		}
		session, err := p.hub.Session(accountID)
		if err != nil {
			p.recordSimpleError(ad, accountID, titleUsed, descUsed, err.Error(), "")
			return "skipped"
		}
		p.setState(func(s *QueueState) { s.Message = "פותח / בודק Facebook..." })
		if err := session.Connect(); err != nil {
			p.recordSimpleError(ad, accountID, titleUsed, descUsed, err.Error(), "")
			return "skipped"
		}
		page, err := session.Page()
		if err != nil {
			p.recordSimpleError(ad, accountID, titleUsed, descUsed, err.Error(), "")
			return "skipped"
		}
		_ = p.store.UpdateStatus(ad.ID, "publishing", "")
		item := automation.Item{Title: titleUsed, Price: ad.Price, CategoryID: ad.CategoryID, Category: ad.Category, Fields: ad.Fields, Condition: ad.Condition, Description: descUsed, Tags: ad.Tags, Images: selectAdImages(ad, settings)}
		started := time.Now()
		err = automation.PrepareItem(page, item)
		if err == nil && dryRun {
			_ = p.store.UpdateStatus(ad.ID, "ready", "")
			p.addHistory(ad, accountID, "preview", "dry-run", "", titleUsed, descUsed, automation.CurrentURL(page), "", started)
			return "preview"
		}
		if err == nil && preview {
			action := p.waitDecision(DecisionState{Type: "preview", AdID: ad.ID, Title: ad.Title, Actions: []string{"publish", "skip", "manual"}}, 0, false)
			switch action {
			case "publish":
			case "skip":
				_ = p.store.UpdateStatus(ad.ID, "skipped", "")
				p.addHistory(ad, accountID, "skipped", "preview-skip", "", titleUsed, descUsed, "", "", started)
				return "skipped"
			case "manual":
				_ = p.store.UpdateStatus(ad.ID, "manual", "")
				p.addHistory(ad, accountID, "manual", "manual", "", titleUsed, descUsed, automation.CurrentURL(page), "", started)
				p.setState(func(s *QueueState) {
					s.Stop = true
					s.Message = "מצב ידני - הדפדפן נשאר פתוח על המודעה"
				})
				return "manual"
			default:
				return "stopped"
			}
		}
		if err == nil {
			if p.license != nil {
				if licErr := p.license.ConsumePublish(1); licErr != nil {
					_ = p.store.UpdateStatus(ad.ID, "error", "רישיון: "+licErr.Error())
					p.setState(func(s *QueueState) { s.Message = "הפרסום נעצר: " + licErr.Error(); s.Stop = true })
					return "stopped"
				}
			}
			preferred := ad.Groups
			if len(preferred) == 0 {
				preferred = settings.FavoriteGroups
			}
			url, pubErr := automation.FinalizeItem(page, automation.PublishOptions{PostToSuggestedGroups: groups, PreferredGroups: preferred, MaxGroups: settings.MaxGroups})
			err = pubErr
			if err == nil {
				_ = p.store.MarkPublished(ad.ID, titleUsed, descUsed, url)
				p.addHistory(ad, accountID, "published", "auto", "", titleUsed, descUsed, url, "", started)
				return "published"
			}
		}
		*totalErrors++
		p.setState(func(s *QueueState) { s.Failed++ })
		screenshot := p.captureError(page, ad.ID)
		_ = p.store.UpdateStatus(ad.ID, "error", err.Error())
		p.addHistory(ad, accountID, "error", "error", err.Error(), titleUsed, descUsed, automation.CurrentURL(page), screenshot, started)
		action := p.waitDecision(DecisionState{Type: "error", AdID: ad.ID, Title: ad.Title, Error: err.Error(), Screenshot: screenshot, Actions: []string{"retry", "skip", "manual"}}, settings.DecisionTimeout, settings.AutoSkipTimeout)
		switch action {
		case "retry":
			continue
		case "manual":
			_ = p.store.UpdateStatus(ad.ID, "manual", err.Error())
			p.setState(func(s *QueueState) { s.Stop = true; s.Message = "מצב ידני - הדפדפן נשאר פתוח" })
			return "manual"
		case "stop":
			return "stopped"
		default:
			_ = p.store.UpdateStatus(ad.ID, "skipped", err.Error())
			p.addHistory(ad, accountID, "skipped", "auto-skip", err.Error(), titleUsed, descUsed, automation.CurrentURL(page), screenshot, time.Now())
			return "skipped"
		}
	}
}

func (p *Publisher) waitDecision(d DecisionState, timeout int, autoSkip bool) string {
	for {
		select {
		case <-p.decisionCh:
		default:
			goto drained
		}
	}
drained:
	if timeout > 0 && autoSkip {
		d.ExpiresAt = time.Now().Add(time.Duration(timeout) * time.Second).Unix()
	}
	p.setState(func(s *QueueState) {
		s.Decision = &d
		if d.Type == "error" {
			s.Message = "שגיאה - ממתין להחלטה"
		} else {
			s.Message = "Preview מוכן - בדוק את Chrome"
		}
	})
	var timer <-chan time.Time
	if timeout > 0 && autoSkip {
		t := time.NewTimer(time.Duration(timeout) * time.Second)
		defer t.Stop()
		timer = t.C
	}
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case action := <-p.decisionCh:
			p.setState(func(s *QueueState) { s.Decision = nil })
			return action
		case <-timer:
			p.setState(func(s *QueueState) {
				s.Decision = nil
				s.Message = "לא התקבלה תשובה - מדלג אוטומטית"
			})
			return "skip"
		case <-tick.C:
			if p.State().Stop {
				p.setState(func(s *QueueState) { s.Decision = nil })
				return "stop"
			}
		}
	}
}

func (p *Publisher) captureError(page interface{ MustScreenshot(...string) []byte }, adID string) string {
	name := fmt.Sprintf("%s-%d.png", adID, time.Now().Unix())
	path := filepath.Join(p.store.ScreenshotDir(), name)
	func() { defer func() { _ = recover() }(); page.MustScreenshot(path) }()
	if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
		return name
	}
	return ""
}

func (p *Publisher) addHistory(ad Ad, accountID, status, action, errText, titleUsed, descUsed, url, screenshot string, started time.Time) {
	h := HistoryEntry{ID: newID(), AdID: ad.ID, AccountID: accountID, Title: ad.Title, Price: ad.Price, Status: status, Action: action, Error: errText, PublishedURL: url, TitleUsed: titleUsed, DescriptionUsed: descUsed, Screenshot: screenshot, StartedAt: started.Format(time.RFC3339), FinishedAt: nowRFC3339(), DurationSeconds: int64(time.Since(started).Seconds())}
	_ = p.store.AddHistory(h)
}

func (p *Publisher) recordSimpleError(ad Ad, accountID, titleUsed, descUsed, errText, screenshot string) {
	_ = p.store.UpdateStatus(ad.ID, "error", errText)
	h := HistoryEntry{ID: newID(), AdID: ad.ID, AccountID: accountID, Title: ad.Title, Price: ad.Price, Status: "error", Action: "connection", Error: errText, TitleUsed: titleUsed, DescriptionUsed: descUsed, Screenshot: screenshot, StartedAt: nowRFC3339(), FinishedAt: nowRFC3339()}
	_ = p.store.AddHistory(h)
	p.setState(func(s *QueueState) { s.Failed++; s.Message = "שגיאה ב-" + ad.Title + ": " + errText })
}

func (p *Publisher) advanceCheckpoint(next int) {
	c := p.store.Checkpoint()
	if c.Active {
		c.NextIndex = next
		_ = p.store.SaveCheckpoint(c)
	}
}

func selectAdImages(ad Ad, settings Settings) []string {
	images := append([]string(nil), ad.Images...)
	if len(images) == 0 {
		return nil
	}

	limit := ad.ImageLimit
	if limit <= 0 || limit > 10 {
		limit = settings.DefaultImageLimit
	}
	if limit <= 0 || limit > 10 {
		limit = 10
	}
	if limit > len(images) {
		limit = len(images)
	}

	blocked := map[int]bool{}
	for _, i := range ad.CoverBlocked {
		if i >= 0 && i < len(images) {
			blocked[i] = true
		}
	}
	eligible := make([]int, 0, len(images))
	for i := range images {
		if !blocked[i] {
			eligible = append(eligible, i)
		}
	}
	// If every image was blocked, keep the current primary/first image as a safe fallback.
	if len(eligible) == 0 {
		eligible = append(eligible, 0)
	}

	coverIndex := ad.PrimaryImage
	if coverIndex < 0 || coverIndex >= len(images) || blocked[coverIndex] {
		coverIndex = eligible[0]
	}
	if settings.ImageRandomCover && len(eligible) > 1 {
		coverIndex = eligible[rand.Intn(len(eligible))]
	}
	cover := images[coverIndex]
	rest := make([]string, 0, len(images)-1)
	for i, img := range images {
		if i != coverIndex {
			rest = append(rest, img)
		}
	}
	if settings.ImageShuffle && len(rest) > 1 {
		rand.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	}
	images = append([]string{cover}, rest...)

	target := limit
	if settings.ImageRandomSubset && limit > 1 {
		minCount := settings.ImageMinCount
		if minCount < 1 {
			minCount = 1
		}
		if minCount > limit {
			minCount = limit
		}
		if limit > minCount {
			target = minCount + rand.Intn(limit-minCount+1)
		}
	}
	if len(images) > target {
		images = images[:target]
	}
	return absolutePaths(images)
}

func absolutePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if abs, err := filepath.Abs(p); err == nil {
			out = append(out, abs)
		} else {
			out = append(out, p)
		}
	}
	return out
}
