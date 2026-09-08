package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type aiRequest struct {
	Mode       string `json:"mode"`
	Text       string `json:"text"`
	Title      string `json:"title,omitempty"`
	Creativity string `json:"creativity,omitempty"`
	Language   string `json:"language,omitempty"`
	Count      int    `json:"count,omitempty"`
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}
type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}
type geminiPart struct {
	Text string `json:"text"`
}
type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

func callGemini(settings Settings, req aiRequest) (string, error) {
	if !settings.GeminiEnabled || strings.TrimSpace(settings.GeminiAPIKey) == "" {
		return "", fmt.Errorf("Gemini is not configured")
	}
	model := strings.TrimSpace(settings.GeminiModel)
	if model == "" {
		model = "gemini-2.5-flash"
	}
	instruction, err := aiInstruction(settings, req)
	if err != nil {
		return "", err
	}
	prompt := instruction + "\n\n" + req.Text
	body, _ := json.Marshal(geminiRequest{Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}}})
	u := "https://generativelanguage.googleapis.com/v1beta/models/" + url.PathEscape(model) + ":generateContent?key=" + url.QueryEscape(settings.GeminiAPIKey)
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out geminiResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("Gemini returned no text")
	}
	return strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text), nil
}
