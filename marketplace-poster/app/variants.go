package main

import (
	"fmt"
	"math/rand"
	"strings"
)

type VariantCreateRequest struct {
	Count       int  `json:"count"`
	UseAIImages bool `json:"use_ai_images"`
	AutoPublish bool `json:"auto_publish"`
	Preview     bool `json:"preview"`
}

func (a *App) createVariantCopies(src Ad, count int, useAIImages bool) ([]Ad, error) {
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}
	settings := a.store.Settings()
	out := make([]Ad, 0, count)
	for i := 0; i < count; i++ {
		copyAd, err := a.store.Duplicate(src.ID)
		if err != nil {
			return out, err
		}
		title, desc := src.Title, src.Description
		if t, d, err := generateCreativePost(settings, src.Title, src.Description); err == nil {
			title, desc = t, d
		}
		tags := append([]string(nil), src.Tags...)
		if tagText, _, err := callAI(settings, aiRequest{Mode: "tags", Text: "כותרת: " + title + "\nתיאור: " + desc + "\nתגיות קיימות: " + strings.Join(tags, ", "), Creativity: "creative", Language: "he", Count: 10}); err == nil {
			tags = mergeTags(tags, parseTags(tagText))
		}
		copyAd.Title, copyAd.Description, copyAd.Tags = title, desc, tags
		copyAd.CategoryID, copyAd.Category, copyAd.Fields = src.CategoryID, src.Category, cloneStringMap(src.Fields)
		copyAd.UseSignature, copyAd.SignatureText = src.UseSignature, src.SignatureText
		if len(copyAd.Images) > 0 {
			blocked := map[int]bool{}
			for _, b := range copyAd.CoverBlocked {
				if b >= 0 && b < len(copyAd.Images) {
					blocked[b] = true
				}
			}
			eligible := []int{}
			for n := range copyAd.Images {
				if !blocked[n] {
					eligible = append(eligible, n)
				}
			}
			if len(eligible) > 0 {
				copyAd.PrimaryImage = eligible[rand.Intn(len(eligible))]
			}
		}
		if settings.ImageRandomSubset && len(copyAd.Images) > 0 {
			min := settings.ImageMinCount
			if min < 1 {
				min = 1
			}
			if min > len(copyAd.Images) {
				min = len(copyAd.Images)
			}
			copyAd.ImageLimit = min + rand.Intn(len(copyAd.Images)-min+1)
		}
		if useAIImages && settings.ImageAIEnabled && len(copyAd.Images) > 0 && imageAIStatus(settings).Ready {
			idx := copyAd.PrimaryImage
			if idx < 0 || idx >= len(copyAd.Images) {
				idx = 0
			}
			input := copyAd.Images[idx]
			output := strings.TrimSuffix(input, filepathExt(input)) + fmt.Sprintf("-ai-%02d.png", i+1)
			if err := generateProductImageVariant(settings, input, output, "Use a different clean commercial presentation while keeping the product truthful."); err == nil {
				shifted := make([]int, 0, len(copyAd.CoverBlocked))
				for _, b := range copyAd.CoverBlocked {
					shifted = append(shifted, b+1)
				}
				copyAd.CoverBlocked = shifted
				copyAd.Images = append([]string{output}, copyAd.Images...)
				copyAd.PrimaryImage = 0
			}
		}
		if err := a.store.UpdateAd(copyAd); err != nil {
			return out, err
		}
		out = append(out, copyAd)
	}
	return out, nil
}

func filepathExt(p string) string {
	idx := strings.LastIndex(p, ".")
	if idx < 0 {
		return ""
	}
	return p[idx:]
}

func cloneStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func mergeTags(a, b []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range append(append([]string{}, a...), b...) {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		k := strings.ToLower(x)
		if !seen[k] {
			seen[k] = true
			out = append(out, x)
		}
		if len(out) >= 12 {
			break
		}
	}
	return out
}
