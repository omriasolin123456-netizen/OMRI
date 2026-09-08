package main

import (
	"bufio"
	"strings"
)

func parseDescriptionText(text string) map[string]string {
	out := map[string]string{}
	s := bufio.NewScanner(strings.NewReader(text))
	inDesc := false
	desc := []string{}
	flush := func() {
		if len(desc) > 0 {
			out["description"] = strings.TrimSpace(strings.Join(desc, "\n"))
			desc = nil
		}
	}
	for s.Scan() {
		line := s.Text()
		lower := strings.ToLower(strings.TrimSpace(line))
		keys := []string{"title:", "price:", "category:", "condition:", "tags:", "description:"}
		matched := ""
		for _, k := range keys {
			if strings.HasPrefix(lower, k) {
				matched = k
				break
			}
		}
		if inDesc && matched != "" {
			flush()
			inDesc = false
		}
		if matched == "" {
			if inDesc {
				desc = append(desc, line)
			}
			continue
		}
		val := strings.TrimSpace(line[len(matched):])
		switch matched {
		case "description:":
			desc = []string{val}
			inDesc = true
		case "title:":
			out["title"] = val
		case "price:":
			out["price"] = val
		case "category:":
			out["category"] = val
		case "condition:":
			out["condition"] = val
		case "tags:":
			out["tags"] = val
		}
	}
	if inDesc {
		flush()
	}
	return out
}
