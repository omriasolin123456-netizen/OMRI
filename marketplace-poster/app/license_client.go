package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const defaultLicenseServerURL = "https://marketplace-poster-license.onrender.com"

type LicenseLocalState struct {
	Token           string `json:"token,omitempty"`
	LicenseID       string `json:"license_id,omitempty"`
	Plan            string `json:"plan,omitempty"`
	Status          string `json:"status,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	MaxDevices      int    `json:"max_devices,omitempty"`
	MaxAccounts     int    `json:"max_accounts,omitempty"`
	PublishLimit    int    `json:"publish_limit,omitempty"`
	PublishUsed     int    `json:"publish_used,omitempty"`
	GraceHours      int    `json:"grace_hours,omitempty"`
	LastValidatedAt string `json:"last_validated_at,omitempty"`
	LastError       string `json:"last_error,omitempty"`
}

type LicensePublicStatus struct {
	Licensed         bool   `json:"licensed"`
	Online           bool   `json:"online"`
	Plan             string `json:"plan,omitempty"`
	Status           string `json:"status,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	MaxDevices       int    `json:"max_devices,omitempty"`
	MaxAccounts      int    `json:"max_accounts,omitempty"`
	PublishLimit     int    `json:"publish_limit,omitempty"`
	PublishUsed      int    `json:"publish_used,omitempty"`
	PublishRemaining int    `json:"publish_remaining,omitempty"`
	GraceHours       int    `json:"grace_hours,omitempty"`
	GraceRemaining   int    `json:"grace_remaining_hours,omitempty"`
	LastValidatedAt  string `json:"last_validated_at,omitempty"`
	Message          string `json:"message,omitempty"`
	ServerURL        string `json:"server_url"`
}

type licenseServerResponse struct {
	OK           bool    `json:"ok"`
	Token        string  `json:"token,omitempty"`
	LicenseID    string  `json:"license_id,omitempty"`
	Plan         string  `json:"plan,omitempty"`
	Status       string  `json:"status,omitempty"`
	ExpiresAt    *string `json:"expires_at,omitempty"`
	MaxDevices   int     `json:"max_devices,omitempty"`
	MaxAccounts  int     `json:"max_accounts,omitempty"`
	PublishLimit int     `json:"publish_limit,omitempty"`
	PublishUsed  int     `json:"publish_used,omitempty"`
	GraceHours   int     `json:"grace_hours,omitempty"`
	Message      string  `json:"message,omitempty"`
	Error        string  `json:"error,omitempty"`
	Remaining    int     `json:"remaining,omitempty"`
}

type LicenseManager struct {
	mu       sync.Mutex
	file     string
	server   string
	client   *http.Client
	deviceID string
	state    LicenseLocalState
}

func NewLicenseManager(dataDir string) *LicenseManager {
	server := strings.TrimSpace(os.Getenv("MARKETPLACE_POSTER_LICENSE_SERVER"))
	if server == "" {
		server = defaultLicenseServerURL
	}
	m := &LicenseManager{
		file:     filepath.Join(dataDir, "license.json"),
		server:   strings.TrimRight(server, "/"),
		client:   &http.Client{Timeout: 12 * time.Second},
		deviceID: localDeviceID(),
	}
	_ = m.load()
	return m
}

func (m *LicenseManager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := os.ReadFile(m.file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &m.state)
}

func (m *LicenseManager) saveLocked() error {
	b, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.file)
}

func (m *LicenseManager) Snapshot() LicenseLocalState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *LicenseManager) Status(fbAccounts int, forceOnline bool) LicensePublicStatus {
	online := false
	if forceOnline || m.shouldValidate() {
		if err := m.Validate(fbAccounts); err == nil {
			online = true
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.publicLocked(online)
}

func (m *LicenseManager) shouldValidate() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Token == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, m.state.LastValidatedAt)
	return err != nil || time.Since(t) > 6*time.Hour
}

func (m *LicenseManager) publicLocked(online bool) LicensePublicStatus {
	st := LicensePublicStatus{
		Online:          online,
		Plan:            m.state.Plan,
		Status:          m.state.Status,
		ExpiresAt:       m.state.ExpiresAt,
		MaxDevices:      m.state.MaxDevices,
		MaxAccounts:     m.state.MaxAccounts,
		PublishLimit:    m.state.PublishLimit,
		PublishUsed:     m.state.PublishUsed,
		GraceHours:      m.state.GraceHours,
		LastValidatedAt: m.state.LastValidatedAt,
		Message:         m.state.LastError,
		ServerURL:       m.server,
	}
	if st.PublishLimit > 0 {
		st.PublishRemaining = st.PublishLimit - st.PublishUsed
		if st.PublishRemaining < 0 {
			st.PublishRemaining = 0
		}
	} else {
		st.PublishRemaining = -1
	}
	st.Licensed = m.allowedLocked()
	if m.state.LastValidatedAt != "" && m.state.GraceHours > 0 {
		if t, err := time.Parse(time.RFC3339, m.state.LastValidatedAt); err == nil {
			remain := time.Duration(m.state.GraceHours)*time.Hour - time.Since(t)
			if remain > 0 {
				st.GraceRemaining = int(remain.Hours())
			}
		}
	}
	return st
}

func (m *LicenseManager) allowedLocked() bool {
	if m.state.Token == "" || m.state.Status != "active" {
		return false
	}
	if m.state.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, m.state.ExpiresAt); err == nil && time.Now().After(t) {
			return false
		}
	}
	if m.state.LastValidatedAt == "" {
		return false
	}
	last, err := time.Parse(time.RFC3339, m.state.LastValidatedAt)
	if err != nil {
		return false
	}
	grace := m.state.GraceHours
	if grace <= 0 {
		grace = 72
	}
	return time.Since(last) <= time.Duration(grace)*time.Hour
}

func (m *LicenseManager) Allowed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.allowedLocked()
}

func (m *LicenseManager) Activate(key string, fbAccounts int) (LicensePublicStatus, error) {
	payload := map[string]any{
		"license_key": strings.TrimSpace(key),
		"device_id":   m.deviceID,
		"device_name": localDeviceName(),
		"app_version": AppVersion,
	}
	var out licenseServerResponse
	if err := m.post("/v1/license/activate", payload, &out); err != nil {
		return LicensePublicStatus{}, err
	}
	m.applyServerResponse(out)
	m.mu.Lock()
	st := m.publicLocked(true)
	m.mu.Unlock()
	return st, nil
}

func (m *LicenseManager) StartTrial(fbAccounts int) (LicensePublicStatus, error) {
	payload := map[string]any{
		"device_id":   m.deviceID,
		"device_name": localDeviceName(),
		"app_version": AppVersion,
	}
	var out licenseServerResponse
	if err := m.post("/v1/trial/start", payload, &out); err != nil {
		return LicensePublicStatus{}, err
	}
	m.applyServerResponse(out)
	m.mu.Lock()
	st := m.publicLocked(true)
	m.mu.Unlock()
	return st, nil
}

func (m *LicenseManager) Validate(fbAccounts int) error {
	m.mu.Lock()
	token := m.state.Token
	m.mu.Unlock()
	if token == "" {
		return fmt.Errorf("no license activated")
	}
	payload := map[string]any{"token": token, "device_id": m.deviceID, "app_version": AppVersion, "fb_accounts": fbAccounts}
	var out licenseServerResponse
	err := m.post("/v1/license/validate", payload, &out)
	if err != nil {
		m.mu.Lock()
		m.state.LastError = err.Error()
		_ = m.saveLocked()
		m.mu.Unlock()
		return err
	}
	m.applyServerResponse(out)
	return nil
}

func (m *LicenseManager) ConsumePublish(amount int) error {
	if amount <= 0 {
		return nil
	}
	m.mu.Lock()
	st := m.state
	m.mu.Unlock()
	if st.Token == "" {
		return fmt.Errorf("license required")
	}
	if st.PublishLimit <= 0 {
		return nil
	}
	payload := map[string]any{"token": st.Token, "device_id": m.deviceID, "kind": "publish", "amount": amount}
	var out licenseServerResponse
	if err := m.post("/v1/usage/consume", payload, &out); err != nil {
		return err
	}
	m.mu.Lock()
	if out.Remaining >= 0 {
		m.state.PublishUsed = m.state.PublishLimit - out.Remaining
	}
	m.state.LastValidatedAt = time.Now().UTC().Format(time.RFC3339)
	m.state.LastError = ""
	_ = m.saveLocked()
	m.mu.Unlock()
	return nil
}

func (m *LicenseManager) Deactivate() error {
	m.mu.Lock()
	token := m.state.Token
	m.mu.Unlock()
	if token != "" {
		var out map[string]any
		_ = m.post("/v1/license/deactivate", map[string]any{"token": token}, &out)
	}
	m.mu.Lock()
	m.state = LicenseLocalState{}
	err := m.saveLocked()
	m.mu.Unlock()
	return err
}

func (m *LicenseManager) CanUseAccountCount(count int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.allowedLocked() && (m.state.MaxAccounts <= 0 || count <= m.state.MaxAccounts)
}

func (m *LicenseManager) MaxAccounts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.MaxAccounts
}

func (m *LicenseManager) applyServerResponse(out licenseServerResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if out.Token != "" {
		m.state.Token = out.Token
	}
	if out.LicenseID != "" {
		m.state.LicenseID = out.LicenseID
	}
	m.state.Plan = out.Plan
	m.state.Status = out.Status
	if out.ExpiresAt != nil {
		m.state.ExpiresAt = *out.ExpiresAt
	} else {
		m.state.ExpiresAt = ""
	}
	m.state.MaxDevices = out.MaxDevices
	m.state.MaxAccounts = out.MaxAccounts
	m.state.PublishLimit = out.PublishLimit
	m.state.PublishUsed = out.PublishUsed
	if out.GraceHours > 0 {
		m.state.GraceHours = out.GraceHours
	} else if m.state.GraceHours == 0 {
		m.state.GraceHours = 72
	}
	m.state.LastValidatedAt = time.Now().UTC().Format(time.RFC3339)
	m.state.LastError = out.Message
	_ = m.saveLocked()
}

func (m *LicenseManager) post(path string, payload any, out any) error {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, m.server+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "MarketplacePoster/"+AppVersion)
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("שרת הרישוי לא זמין: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e licenseServerResponse
		_ = json.Unmarshal(body, &e)
		msg := strings.TrimSpace(e.Error)
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("%s", msg)
	}
	if len(body) > 0 && out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("תגובה לא תקינה משרת הרישוי: %w", err)
		}
	}
	return nil
}

func localDeviceName() string {
	h, _ := os.Hostname()
	if strings.TrimSpace(h) == "" {
		return "Windows PC"
	}
	return h
}

func localDeviceID() string {
	parts := []string{runtime.GOOS, runtime.GOARCH}
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("reg", "query", `HKLM\\SOFTWARE\\Microsoft\\Cryptography`, "/v", "MachineGuid").CombinedOutput(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), "machineguid") {
					fields := strings.Fields(line)
					if len(fields) > 0 {
						parts = append(parts, fields[len(fields)-1])
					}
				}
			}
		}
	}
	h, _ := os.Hostname()
	parts = append(parts, h)
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}
