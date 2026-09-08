package main

import (
	"math/rand"
	"regexp"
	"strings"
)

var spinRE = regexp.MustCompile(`\{([^{}]+)\}`)

func chooseAdText(ad Ad) (string, string) {
	mode := ad.VariantMode
	if mode == "" {
		mode = "sequence"
	}
	titles := append([]string{ad.Title}, ad.TitleVariants...)
	descs := append([]string{ad.Description}, ad.DescriptionVariants...)
	pick := func(vals []string) string {
		clean := []string{}
		for _, v := range vals {
			if strings.TrimSpace(v) != "" {
				clean = append(clean, v)
			}
		}
		if len(clean) == 0 {
			return ""
		}
		switch mode {
		case "random":
			return clean[rand.Intn(len(clean))]
		case "first":
			return clean[0]
		default:
			return clean[ad.PublishCount%len(clean)]
		}
	}
	return resolveSpintax(pick(titles)), resolveSpintax(pick(descs))
}

func resolveSpintax(s string) string {
	for i := 0; i < 20; i++ {
		loc := spinRE.FindStringSubmatchIndex(s)
		if loc == nil {
			break
		}
		body := s[loc[2]:loc[3]]
		opts := strings.Split(body, "|")
		choice := ""
		if len(opts) > 0 {
			choice = opts[rand.Intn(len(opts))]
		}
		s = s[:loc[0]] + choice + s[loc[1]:]
	}
	return s
}

func renderTemplate(src string, vars map[string]string) string {
	out := src
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+strings.ToUpper(strings.TrimSpace(k))+"}}", v)
		out = strings.ReplaceAll(out, "{{"+strings.TrimSpace(k)+"}}", v)
	}
	return out
}
