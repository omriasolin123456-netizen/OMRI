package main

import "time"

const AppVersion = "2.5.0"

type Ad struct {
	ID                  string            `json:"id"`
	Title               string            `json:"title"`
	TitleVariants       []string          `json:"title_variants,omitempty"`
	Description         string            `json:"description"`
	DescriptionVariants []string          `json:"description_variants,omitempty"`
	VariantMode         string            `json:"variant_mode,omitempty"` // first, sequence, random
	Price               string            `json:"price"`
	CategoryID          string            `json:"category_id,omitempty"`
	Category            string            `json:"category"`
	Fields              map[string]string `json:"fields,omitempty"`
	Condition           string            `json:"condition"`
	Tags                []string          `json:"tags"`
	Images              []string          `json:"images"`
	PrimaryImage        int               `json:"primary_image,omitempty"`
	CoverBlocked        []int             `json:"cover_blocked,omitempty"`
	ImageLimit          int               `json:"image_limit,omitempty"`
	UseSignature        *bool             `json:"use_signature,omitempty"`
	SignatureText       string            `json:"signature_text,omitempty"`
	Groups              []string          `json:"groups,omitempty"`
	AccountID           string            `json:"account_id,omitempty"`
	Status              string            `json:"status"`
	Error               string            `json:"error,omitempty"`
	Archived            bool              `json:"archived,omitempty"`
	PublishCount        int               `json:"publish_count,omitempty"`
	LastTitleUsed       string            `json:"last_title_used,omitempty"`
	LastDescriptionUsed string            `json:"last_description_used,omitempty"`
	PublishedURL        string            `json:"published_url,omitempty"`
	CreatedAt           string            `json:"created_at"`
	UpdatedAt           string            `json:"updated_at,omitempty"`
	PublishedAt         string            `json:"published_at,omitempty"`
}

type HistoryEntry struct {
	ID              string `json:"id"`
	AdID            string `json:"ad_id"`
	AccountID       string `json:"account_id,omitempty"`
	Title           string `json:"title"`
	Price           string `json:"price,omitempty"`
	Status          string `json:"status"` // published,error,skipped,manual,preview
	Action          string `json:"action,omitempty"`
	Error           string `json:"error,omitempty"`
	PublishedURL    string `json:"published_url,omitempty"`
	TitleUsed       string `json:"title_used,omitempty"`
	DescriptionUsed string `json:"description_used,omitempty"`
	Screenshot      string `json:"screenshot,omitempty"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at,omitempty"`
	DurationSeconds int64  `json:"duration_seconds,omitempty"`
}

type Template struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category,omitempty"`
	Condition   string   `json:"condition,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CreatedAt   string   `json:"created_at"`
}

type Account struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type Settings struct {
	DailyLimit              int      `json:"daily_limit"`
	RunLimit                int      `json:"run_limit"`
	MinDelay                int      `json:"min_delay"`
	MaxDelay                int      `json:"max_delay"`
	MaxErrors               int      `json:"max_errors"`
	DecisionTimeout         int      `json:"decision_timeout"`
	AutoSkipTimeout         bool     `json:"auto_skip_timeout"`
	PreviewBeforePublish    bool     `json:"preview_before_publish"`
	PostToGroups            bool     `json:"post_to_groups"`
	MaxGroups               int      `json:"max_groups"`
	FavoriteGroups          []string `json:"favorite_groups,omitempty"`
	DefaultCategory         string   `json:"default_category,omitempty"`
	DefaultCondition        string   `json:"default_condition"`
	DefaultImageLimit       int      `json:"default_image_limit"`
	VariantMode             string   `json:"variant_mode"`
	ActiveAccountID         string   `json:"active_account_id,omitempty"`
	Notifications           bool     `json:"notifications"`
	DarkMode                bool     `json:"dark_mode"`
	LocalAIEnabled          bool     `json:"local_ai_enabled"`
	LocalAIAutoStart        bool     `json:"local_ai_auto_start"`
	LocalAIBaseURL          string   `json:"local_ai_base_url,omitempty"`
	LocalAIModel            string   `json:"local_ai_model,omitempty"`
	LocalAIPath             string   `json:"local_ai_path,omitempty"`
	AIModelMode             string   `json:"ai_model_mode,omitempty"` // auto, manual
	AutoInstallBestModel    bool     `json:"auto_install_best_model"`
	AICreativeMode          string   `json:"ai_creative_mode,omitempty"` // conservative, balanced, creative
	AIHebrewOnly            bool     `json:"ai_hebrew_only"`
	AIVariantCount          int      `json:"ai_variant_count"`
	AutoCreativeText        bool     `json:"auto_creative_text"`
	ImageShuffle            bool     `json:"image_shuffle"`
	ImageRandomCover        bool     `json:"image_random_cover"`
	ImageRandomSubset       bool     `json:"image_random_subset"`
	ImageMinCount           int      `json:"image_min_count"`
	DefaultSignatureEnabled bool     `json:"default_signature_enabled"`
	DefaultSignatureText    string   `json:"default_signature_text,omitempty"`
	DefaultPhone            string   `json:"default_phone,omitempty"`
	PhoneEmoji              bool     `json:"phone_emoji"`
	ImageAIEnabled          bool     `json:"image_ai_enabled"`
	ImageAIRuntimePath      string   `json:"image_ai_runtime_path,omitempty"`
	ImageAIModelPath        string   `json:"image_ai_model_path,omitempty"`
	ImageAIVAEPath          string   `json:"image_ai_vae_path,omitempty"`
	ImageAILLMPath          string   `json:"image_ai_llm_path,omitempty"`
	ImageAILLMVisionPath    string   `json:"image_ai_llm_vision_path,omitempty"`
	GeminiEnabled           bool     `json:"gemini_enabled"`
	GeminiAPIKey            string   `json:"gemini_api_key,omitempty"`
	GeminiModel             string   `json:"gemini_model,omitempty"`
	UpdateManifestURL       string   `json:"update_manifest_url,omitempty"`
	CheckUpdatesOnStart     bool     `json:"check_updates_on_start"`
	PINHash                 string   `json:"pin_hash,omitempty"`
}

type QueueCheckpoint struct {
	Active               bool     `json:"active"`
	IDs                  []string `json:"ids,omitempty"`
	NextIndex            int      `json:"next_index"`
	MinDelay             int      `json:"min_delay"`
	MaxDelay             int      `json:"max_delay"`
	Groups               bool     `json:"groups"`
	PreviewBeforePublish bool     `json:"preview_before_publish"`
	DryRun               bool     `json:"dry_run"`
	AccountID            string   `json:"account_id,omitempty"`
	StartedAt            string   `json:"started_at,omitempty"`
}

type AppData struct {
	Ads        []Ad            `json:"ads"`
	History    []HistoryEntry  `json:"history"`
	Templates  []Template      `json:"templates"`
	Accounts   []Account       `json:"accounts"`
	Settings   Settings        `json:"settings"`
	Checkpoint QueueCheckpoint `json:"checkpoint"`
	SavedAt    string          `json:"saved_at"`
}

type DashboardStats struct {
	TotalAds       int     `json:"total_ads"`
	Waiting        int     `json:"waiting"`
	PublishedToday int     `json:"published_today"`
	FailedToday    int     `json:"failed_today"`
	SkippedToday   int     `json:"skipped_today"`
	AvgSeconds     float64 `json:"avg_seconds"`
	Archived       int     `json:"archived"`
}

func defaultSettings() Settings {
	return Settings{
		DailyLimit:              15,
		RunLimit:                5,
		MinDelay:                45,
		MaxDelay:                180,
		MaxErrors:               3,
		DecisionTimeout:         60,
		AutoSkipTimeout:         true,
		PreviewBeforePublish:    false,
		PostToGroups:            false,
		MaxGroups:               3,
		DefaultCondition:        "חדש",
		DefaultImageLimit:       10,
		VariantMode:             "sequence",
		Notifications:           true,
		DarkMode:                false,
		LocalAIEnabled:          true,
		LocalAIAutoStart:        true,
		LocalAIBaseURL:          defaultLocalAIBaseURL,
		LocalAIModel:            defaultLocalAIModel,
		AIModelMode:             "auto",
		AutoInstallBestModel:    true,
		AICreativeMode:          "creative",
		AIHebrewOnly:            true,
		AIVariantCount:          5,
		AutoCreativeText:        true,
		ImageShuffle:            true,
		ImageRandomCover:        true,
		ImageRandomSubset:       true,
		ImageMinCount:           3,
		DefaultSignatureEnabled: false,
		DefaultSignatureText:    "",
		DefaultPhone:            "",
		PhoneEmoji:              false,
		ImageAIEnabled:          false,
		GeminiEnabled:           false,
		GeminiModel:             "gemini-2.5-flash",
		UpdateManifestURL:       "https://raw.githubusercontent.com/omriasolin123456-netizen/OMRI/marketplace-poster-updates/release/manifest.json",
		CheckUpdatesOnStart:     true,
	}
}

func nowRFC3339() string { return time.Now().Format(time.RFC3339) }
