package automation

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

func PrepareItem(page *rod.Page, item Item) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("prepare failed: %v", r)
		}
	}()
	if page == nil {
		return fmt.Errorf("Facebook page is not available")
	}
	if len(item.Images) == 0 {
		return fmt.Errorf("at least one image is required")
	}
	createURL := "https://www.facebook.com/marketplace/create/item"
	if item.CategoryID == "vehicles" || item.CategoryID == "motorcycles" {
		createURL = "https://www.facebook.com/marketplace/create/vehicle"
	}
	page.MustNavigate(createURL).MustWaitLoad()
	time.Sleep(2 * time.Second)
	if hasLimitReached(page) {
		return fmt.Errorf("Facebook reports that the listing limit was reached")
	}
	upload := page.MustElement(`input[type="file"]`)
	upload.MustSetFiles(item.Images...)
	time.Sleep(1200 * time.Millisecond)
	fillForm(page, item)
	return nil
}

func FinalizeItem(page *rod.Page, opts PublishOptions) (publishedURL string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("publish failed: %v", r)
		}
	}()
	if page == nil {
		return "", fmt.Errorf("Facebook page is not available")
	}
	if clickButton(page, []string{"Next", "הבא"}) {
		waitForAudienceOrPublish(page, 60*time.Second)
	}
	if opts.PostToSuggestedGroups {
		selectSuggestedGroups(page, opts.PreferredGroups, opts.MaxGroups)
	}
	if !clickButton(page, []string{"Publish", "פרסום", "פרסם"}) {
		return "", fmt.Errorf("publish button was not found")
	}
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		url := page.MustInfo().URL
		if strings.Contains(url, "/marketplace/item/") {
			return url, nil
		}
		if strings.Contains(url, "/marketplace/you/selling") {
			return url, nil
		}
		html := strings.ToLower(page.MustHTML())
		if strings.Contains(html, "something isn't working") || strings.Contains(html, "משהו השתבש") {
			return "", fmt.Errorf("Facebook reported a publishing error")
		}
		if strings.Contains(html, "limit reached") || strings.Contains(html, "הגעת למגבלה") {
			return "", fmt.Errorf("Facebook reports that the listing limit was reached")
		}
		time.Sleep(2 * time.Second)
	}
	return "", fmt.Errorf("timed out while waiting for Facebook to finish publishing")
}

func PublishItem(page *rod.Page, item Item, opts PublishOptions) (string, error) {
	if err := PrepareItem(page, item); err != nil {
		return "", err
	}
	return FinalizeItem(page, opts)
}

func CurrentURL(page *rod.Page) string {
	if page == nil {
		return ""
	}
	defer func() { _ = recover() }()
	return page.MustInfo().URL
}

func Screenshot(page *rod.Page, path string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("screenshot failed: %v", r)
		}
	}()
	if page == nil {
		return fmt.Errorf("page not available")
	}
	page.MustScreenshot(path)
	return nil
}

func hasLimitReached(page *rod.Page) bool {
	html := strings.ToLower(page.MustHTML())
	return strings.Contains(html, "limit reached") || strings.Contains(html, "הגעת למגבלה")
}

func fillForm(page *rod.Page, item Item) {
	labels := page.MustElements("label")
	matched := map[string]bool{}
	for _, label := range labels {
		text := normalize(label.MustText())
		switch {
		case hasAny(text, "title", "כותרת"):
			tryInput(label, item.Title)
			matched["title"] = true
		case hasAny(text, "price", "מחיר"):
			tryInput(label, item.Price)
			matched["price"] = true
		case hasAny(text, "category", "קטגוריה"):
			if strings.TrimSpace(item.Category) != "" {
				selectDrawerValue(page, label, categoryAliases(item.Category))
				matched["category"] = true
			}
		case hasAny(text, "condition", "מצב"):
			if strings.TrimSpace(item.Condition) != "" {
				selectDrawerValue(page, label, conditionAliases(item.Condition))
				matched["condition"] = true
			}
		case hasAny(text, "description", "תיאור"):
			tryInput(label, item.Description)
			matched["description"] = true
		case hasAny(text, "year", "שנה"):
			fillExtraField(page, label, item.Fields, "year")
		case hasAny(text, "make", "manufacturer", "יצרן"):
			fillExtraField(page, label, item.Fields, "make")
		case hasAny(text, "model", "דגם"):
			fillExtraField(page, label, item.Fields, "model")
		case hasAny(text, "mileage", "kilometers", "kilometres", "קילומטר", "קילומטראז"):
			fillExtraField(page, label, item.Fields, "mileage_km")
		case hasAny(text, "transmission", "תיבת הילוכים", "גיר"):
			fillExtraField(page, label, item.Fields, "transmission")
		case hasAny(text, "fuel", "דלק"):
			fillExtraField(page, label, item.Fields, "fuel")
		case hasAny(text, "color", "colour", "צבע"):
			fillExtraField(page, label, item.Fields, "color")
		case hasAny(text, "owners", "previous owners", "יד"):
			fillExtraField(page, label, item.Fields, "owners")
		case hasAny(text, "product tags", "תגיות מוצר", "תגיות"):
			for _, tag := range item.Tags {
				tag = strings.TrimSpace(tag)
				if tag == "" {
					continue
				}
				tryInput(label, tag)
				_ = page.Keyboard.Press(input.Enter)
			}
		}
	}
	if !matched["title"] || !matched["price"] {
		panic("required Marketplace fields were not detected; Facebook may have changed the form")
	}
}

func tryInput(el *rod.Element, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	defer func() { _ = recover() }()
	el.MustInput(value)
}

func selectDrawerValue(page *rod.Page, trigger *rod.Element, aliases []string) {
	trigger.MustClick()
	time.Sleep(700 * time.Millisecond)
	for attempt := 0; attempt < 6; attempt++ {
		for _, selector := range []string{`div[role="option"]`, `div[role="button"]`} {
			options := page.MustElements(selector)
			for _, option := range options {
				text := normalize(option.MustText())
				if text == "" {
					continue
				}
				for _, alias := range aliases {
					a := normalize(alias)
					if text == a || strings.Contains(text, a) {
						option.MustClick()
						time.Sleep(500 * time.Millisecond)
						return
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	panic("could not select option: " + strings.Join(aliases, " / "))
}

func conditionAliases(value string) []string {
	v := normalize(value)
	switch v {
	case "חדש", "new":
		return []string{"חדש", "New"}
	case "משומש - כמו חדש", "משומש – כמו חדש", "used - like new", "used – like new":
		return []string{"משומש - כמו חדש", "משומש – כמו חדש", "Used - Like New", "Used – Like New"}
	case "משומש - מצב טוב", "משומש – מצב טוב", "used - good", "used – good":
		return []string{"משומש - מצב טוב", "משומש – מצב טוב", "Used - Good", "Used – Good"}
	case "משומש - מצב סביר", "משומש – מצב סביר", "used - fair", "used – fair":
		return []string{"משומש - מצב סביר", "משומש – מצב סביר", "Used - Fair", "Used – Fair"}
	default:
		return []string{value}
	}
}
func categoryAliases(value string) []string {
	v := normalize(value)
	switch v {
	case "ריהוט", "furniture":
		return []string{"ריהוט", "Furniture"}
	case "משחקי וידאו", "video games":
		return []string{"משחקי וידאו", "Video Games"}
	case "מוצרי חשמל", "appliances":
		return []string{"מוצרי חשמל", "Appliances"}
	case "אלקטרוניקה", "electronics":
		return []string{"אלקטרוניקה", "Electronics"}
	case "מחשבים", "computers":
		return []string{"מחשבים", "Computers"}
	case "טלפונים ואביזרים", "phones":
		return []string{"טלפונים ואביזרים", "Cell Phones", "Mobile Phones"}
	case "בית וגינה", "home":
		return []string{"בית וגינה", "Home & Garden"}
	case "ספורט וכושר", "sports":
		return []string{"ספורט וכושר", "Sporting Goods"}
	case "ביגוד ואופנה", "clothing":
		return []string{"ביגוד ואופנה", "Clothing"}
	default:
		return []string{value}
	}
}

func fillExtraField(page *rod.Page, label *rod.Element, fields map[string]string, key string) {
	v := strings.TrimSpace(fields[key])
	if v == "" {
		return
	}
	// First try as a regular input; if the field is a drawer/select, fall back to option selection.
	func() { defer func() { _ = recover() }(); label.MustInput(v) }()
	text := normalize(label.MustText())
	if strings.Contains(text, "transmission") || strings.Contains(text, "תיבת הילוכים") || strings.Contains(text, "fuel") || strings.Contains(text, "דלק") {
		func() { defer func() { _ = recover() }(); selectDrawerValue(page, label, []string{v}) }()
	}
}

func clickButton(page *rod.Page, names []string) bool {
	buttons := page.MustElements(`div[role="button"]`)
	for _, button := range buttons {
		text := normalize(button.MustText())
		aria := ""
		if a, err := button.Attribute("aria-label"); err == nil && a != nil {
			aria = normalize(*a)
		}
		for _, name := range names {
			n := normalize(name)
			if text == n || aria == n {
				button.MustClick()
				time.Sleep(800 * time.Millisecond)
				return true
			}
		}
	}
	return false
}
func waitForAudienceOrPublish(page *rod.Page, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		u := page.MustInfo().URL
		if strings.Contains(u, "step=audience") || hasButton(page, []string{"Publish", "פרסום", "פרסם"}) {
			return
		}
		time.Sleep(time.Second)
	}
}
func hasButton(page *rod.Page, names []string) bool {
	buttons := page.MustElements(`div[role="button"]`)
	for _, b := range buttons {
		text := normalize(b.MustText())
		aria := ""
		if a, e := b.Attribute("aria-label"); e == nil && a != nil {
			aria = normalize(*a)
		}
		for _, n := range names {
			n = normalize(n)
			if text == n || aria == n {
				return true
			}
		}
	}
	return false
}

func selectSuggestedGroups(page *rod.Page, preferred []string, maxGroups int) {
	wanted := make([]string, 0, len(preferred))
	for _, x := range preferred {
		if strings.TrimSpace(x) != "" {
			wanted = append(wanted, normalize(x))
		}
	}
	// Never select Facebook's suggested groups blindly. Only groups the user
	// explicitly saved or selected are eligible. Zero means all of those groups.
	if len(wanted) == 0 {
		return
	}
	if maxGroups <= 0 || maxGroups > len(wanted) {
		maxGroups = len(wanted)
	}
	clicked := 0
	for _, box := range page.MustElements(`div[role="checkbox"]`) {
		if clicked >= maxGroups {
			return
		}
		text := normalize(box.MustText())
		if text == "" || strings.Contains(text, "marketplace") || strings.Contains(text, "מרקטפלייס") {
			continue
		}
		match := false
		for _, w := range wanted {
			if strings.Contains(text, w) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		func() { defer func() { _ = recover() }(); box.MustClick() }()
		clicked++
		time.Sleep(150 * time.Millisecond)
	}
}

var whitespace = regexp.MustCompile(`\s+`)

func normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	return whitespace.ReplaceAllString(s, " ")
}
func hasAny(s string, values ...string) bool {
	for _, v := range values {
		if strings.Contains(s, normalize(v)) {
			return true
		}
	}
	return false
}
