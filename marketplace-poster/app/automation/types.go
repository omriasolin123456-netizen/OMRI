package automation

type Item struct {
	Title       string            `json:"title"`
	Price       string            `json:"price"`
	CategoryID  string            `json:"category_id,omitempty"`
	Category    string            `json:"category"`
	Fields      map[string]string `json:"fields,omitempty"`
	Condition   string            `json:"condition"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Images      []string          `json:"images"`
}

type PublishOptions struct {
	PostToSuggestedGroups bool
	PreferredGroups       []string
	MaxGroups             int
}
