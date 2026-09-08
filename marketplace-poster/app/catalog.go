package main

import "strings"

type CategoryField struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"` // text, number, select
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"`
}

type CategoryDefinition struct {
	ID      string          `json:"id"`
	Label   string          `json:"label"`
	Aliases []string        `json:"aliases,omitempty"`
	Fields  []CategoryField `json:"fields,omitempty"`
}

var marketplaceCategories = []CategoryDefinition{
	{ID: "furniture", Label: "ריהוט", Aliases: []string{"רהיטים", "שולחן", "כיסא", "ארון", "ספה", "מיטה"}, Fields: []CategoryField{{Key: "material", Label: "חומר", Type: "text"}, {Key: "color", Label: "צבע", Type: "text"}, {Key: "dimensions", Label: "מידות", Type: "text"}}},
	{ID: "home", Label: "בית וגינה", Aliases: []string{"לבית", "גינה", "עיצוב הבית", "כלי בית"}},
	{ID: "appliances", Label: "מוצרי חשמל", Aliases: []string{"מקרר", "מכונת כביסה", "תנור", "מדיח", "מזגן"}, Fields: []CategoryField{{Key: "brand", Label: "מותג", Type: "text"}, {Key: "model", Label: "דגם", Type: "text"}}},
	{ID: "electronics", Label: "אלקטרוניקה", Aliases: []string{"אלקטרוני", "טלוויזיה", "מסך", "אוזניות", "מצלמה"}, Fields: []CategoryField{{Key: "brand", Label: "מותג", Type: "text"}, {Key: "model", Label: "דגם", Type: "text"}}},
	{ID: "computers", Label: "מחשבים", Aliases: []string{"מחשב", "לפטופ", "נייד", "מחשב נייח", "מסך מחשב"}, Fields: []CategoryField{{Key: "brand", Label: "מותג", Type: "text"}, {Key: "model", Label: "דגם", Type: "text"}, {Key: "storage", Label: "אחסון", Type: "text"}, {Key: "memory", Label: "זיכרון RAM", Type: "text"}}},
	{ID: "phones", Label: "טלפונים ואביזרים", Aliases: []string{"טלפון", "סמארטפון", "אייפון", "אנדרואיד"}, Fields: []CategoryField{{Key: "brand", Label: "מותג", Type: "text"}, {Key: "model", Label: "דגם", Type: "text"}, {Key: "storage", Label: "נפח אחסון", Type: "text"}, {Key: "color", Label: "צבע", Type: "text"}}},
	{ID: "video_games", Label: "משחקי וידאו", Aliases: []string{"משחקי מחשב", "קונסולה", "פלייסטיישן", "אקסבוקס", "נינטנדו", "ps5", "xbox"}, Fields: []CategoryField{{Key: "platform", Label: "פלטפורמה", Type: "text"}, {Key: "title", Label: "שם המשחק / המוצר", Type: "text"}}},
	{ID: "sports", Label: "ספורט וכושר", Aliases: []string{"כושר", "ציוד ספורט", "אימון"}},
	{ID: "clothing", Label: "ביגוד ואופנה", Aliases: []string{"בגדים", "נעליים", "אופנה"}, Fields: []CategoryField{{Key: "size", Label: "מידה", Type: "text"}, {Key: "color", Label: "צבע", Type: "text"}, {Key: "brand", Label: "מותג", Type: "text"}}},
	{ID: "baby", Label: "תינוקות וילדים", Aliases: []string{"ילדים", "תינוק", "צעצועים"}},
	{ID: "collectibles", Label: "אספנות ותחביבים", Aliases: []string{"אספנות", "תחביב", "פריט אספנות"}},
	{ID: "tools", Label: "כלי עבודה", Aliases: []string{"עבודה", "נגרות", "מקדחה", "מסור"}, Fields: []CategoryField{{Key: "brand", Label: "מותג", Type: "text"}, {Key: "model", Label: "דגם", Type: "text"}}},
	{ID: "vehicles", Label: "רכבים", Aliases: []string{"רכב", "מכונית", "אוטו", "car", "vehicle"}, Fields: []CategoryField{
		{Key: "year", Label: "שנה", Type: "number", Required: true},
		{Key: "make", Label: "יצרן", Type: "text", Required: true},
		{Key: "model", Label: "דגם", Type: "text", Required: true},
		{Key: "mileage_km", Label: "קילומטר", Type: "number", Required: true},
		{Key: "transmission", Label: "תיבת הילוכים", Type: "select", Options: []string{"אוטומט", "ידני", "רובוטי"}},
		{Key: "fuel", Label: "דלק", Type: "select", Options: []string{"בנזין", "דיזל", "היברידי", "חשמלי"}},
		{Key: "color", Label: "צבע", Type: "text"},
		{Key: "owners", Label: "יד", Type: "number"},
	}},
	{ID: "motorcycles", Label: "אופנועים וקטנועים", Aliases: []string{"אופנוע", "קטנוע"}, Fields: []CategoryField{{Key: "year", Label: "שנה", Type: "number"}, {Key: "make", Label: "יצרן", Type: "text"}, {Key: "model", Label: "דגם", Type: "text"}, {Key: "mileage_km", Label: "קילומטר", Type: "number"}}},
	{ID: "other", Label: "אחר", Aliases: []string{"כללי", "אחר"}},
}

func categoryByID(id string) (CategoryDefinition, bool) {
	id = strings.TrimSpace(strings.ToLower(id))
	for _, c := range marketplaceCategories {
		if c.ID == id {
			return c, true
		}
	}
	return CategoryDefinition{}, false
}

func categoryLabel(id, fallback string) string {
	if c, ok := categoryByID(id); ok {
		return c.Label
	}
	return strings.TrimSpace(fallback)
}

func inferCategory(query string) CategoryDefinition {
	q := strings.ToLower(strings.TrimSpace(query))
	best := marketplaceCategories[len(marketplaceCategories)-1]
	for _, c := range marketplaceCategories {
		if strings.Contains(q, strings.ToLower(c.Label)) || strings.Contains(q, strings.ToLower(c.ID)) {
			return c
		}
		for _, a := range c.Aliases {
			if strings.Contains(q, strings.ToLower(a)) {
				return c
			}
		}
	}
	return best
}
