package automation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type BrowserSession struct {
	mu         sync.Mutex
	browser    *rod.Browser
	page       *rod.Page
	profileDir string
	status     string
	lastErr    string
}

func NewBrowserSession(profileDir string) *BrowserSession {
	return &BrowserSession{profileDir: profileDir, status: "disconnected"}
}

func (s *BrowserSession) Status() (status, lastErr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status, s.lastErr
}

func (s *BrowserSession) setStatus(status, lastErr string) {
	s.mu.Lock()
	s.status = status
	s.lastErr = lastErr
	s.mu.Unlock()
}

func (s *BrowserSession) Connect() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("browser error: %v", r)
			s.setStatus("error", err.Error())
		}
	}()
	s.mu.Lock()
	if s.browser != nil && s.page != nil {
		page := s.page
		s.mu.Unlock()
		alive := false
		func() {
			defer func() { _ = recover() }()
			_ = page.MustInfo().URL
			alive = true
		}()
		if alive {
			s.setStatus("connected", "")
			return nil
		}
		s.Close()
		s.mu.Lock()
	}
	s.status = "connecting"
	s.lastErr = ""
	s.mu.Unlock()

	if err := os.MkdirAll(s.profileDir, 0o755); err != nil {
		s.setStatus("error", err.Error())
		return err
	}

	// Prefer the browser already installed on Windows. This avoids launching a
	// separately downloaded Chromium build and keeps DevTools strictly on loopback,
	// which prevents unnecessary Windows Firewall prompts.
	launch := launcher.New().
		Headless(false).
		Devtools(false).
		UserDataDir(filepath.Clean(s.profileDir)).
		Set("remote-debugging-address", "127.0.0.1")
	if bin, found := launcher.LookPath(); found && strings.TrimSpace(bin) != "" {
		launch = launch.Bin(bin)
	}

	controlURL, err := launch.Launch()
	if err != nil {
		s.setStatus("error", err.Error())
		return fmt.Errorf("failed to launch Chrome: %w", err)
	}

	browser := rod.New().ControlURL(controlURL).MustConnect().NoDefaultDevice()

	page := browser.MustPage("https://www.facebook.com/")
	page.MustWindowMaximize()

	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if isFacebookLoggedIn(page) {
			s.mu.Lock()
			s.browser = browser
			s.page = page
			s.status = "connected"
			s.lastErr = ""
			s.mu.Unlock()
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	_ = browser.Close()
	err = fmt.Errorf("Facebook login was not completed within 3 minutes")
	s.setStatus("error", err.Error())
	return err
}

func isFacebookLoggedIn(page *rod.Page) bool {
	if page == nil {
		return false
	}
	url := ""
	func() {
		defer func() { _ = recover() }()
		url = page.MustInfo().URL
	}()
	if strings.Contains(url, "/login") {
		return false
	}

	selectors := []string{
		`div[aria-label="Your profile"]`,
		`div[aria-label="הפרופיל שלך"]`,
		`[role="navigation"]`,
	}
	for _, sel := range selectors {
		ok := false
		func() {
			defer func() { _ = recover() }()
			ok = page.MustHas(sel)
		}()
		if ok {
			return true
		}
	}
	return false
}

func (s *BrowserSession) Page() (*rod.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.page == nil || s.browser == nil {
		return nil, fmt.Errorf("Facebook is not connected")
	}
	return s.page, nil
}

func (s *BrowserSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.browser != nil {
		func() {
			defer func() { _ = recover() }()
			s.browser.MustClose()
		}()
	}
	s.browser = nil
	s.page = nil
	s.status = "disconnected"
	s.lastErr = ""
}
