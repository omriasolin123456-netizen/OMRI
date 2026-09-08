package main

import (
	"strings"
	"unicode"
)

var digitEmoji = map[rune]string{'0': "0️⃣", '1': "1️⃣", '2': "2️⃣", '3': "3️⃣", '4': "4️⃣", '5': "5️⃣", '6': "6️⃣", '7': "7️⃣", '8': "8️⃣", '9': "9️⃣"}

func phoneEmoji(s string) string {
	var b strings.Builder
	for _, r := range s {
		if e, ok := digitEmoji[r]; ok {
			b.WriteString(e)
			continue
		}
		if unicode.IsSpace(r) || r == '-' || r == '(' || r == ')' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func decorateDescription(ad Ad, settings Settings, description string) string {
	use := settings.DefaultSignatureEnabled
	if ad.UseSignature != nil {
		use = *ad.UseSignature
	}
	if !use {
		return strings.TrimSpace(description)
	}
	sig := strings.TrimSpace(ad.SignatureText)
	if sig == "" {
		sig = strings.TrimSpace(settings.DefaultSignatureText)
	}
	phone := strings.TrimSpace(settings.DefaultPhone)
	if settings.PhoneEmoji && phone != "" {
		phone = phoneEmoji(phone)
	}
	if strings.Contains(sig, "{{PHONE}}") {
		sig = strings.ReplaceAll(sig, "{{PHONE}}", phone)
		phone = ""
	}
	parts := []string{strings.TrimSpace(description)}
	if sig != "" {
		parts = append(parts, sig)
	}
	if phone != "" {
		parts = append(parts, phone)
	}
	return strings.Join(parts, "\n\n")
}
