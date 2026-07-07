package config

// ConfigMeta defines metadata for a configuration item
type ConfigMeta struct {
	Type      string // "string", "bool", "int", "float", "json"
	Default   any
	Encrypted bool
}

// ConfigRegistry defines all available configuration items
var ConfigRegistry = map[string]ConfigMeta{
	// API settings
	"api.token":   {Type: "string", Default: "", Encrypted: false},
	"api.enabled": {Type: "bool", Default: false, Encrypted: false},

	// Memos webhook sync settings
	"memos.enabled":       {Type: "bool", Default: false, Encrypted: false},
	"memos.webhook_token": {Type: "string", Default: "", Encrypted: true},
	"memos.base_url":      {Type: "string", Default: "", Encrypted: false},

	// AI settings (unified API key and base URL)
	"ai.enabled":               {Type: "bool", Default: false, Encrypted: false},
	"ai.api_key":               {Type: "string", Default: "", Encrypted: true},
	"ai.base_url":              {Type: "string", Default: "", Encrypted: false},
	"ai.chat_model":            {Type: "string", Default: "", Encrypted: false},
	"ai.embedding_model":       {Type: "string", Default: "", Encrypted: false},
	"ai.vectors_built_at":      {Type: "string", Default: "", Encrypted: false},
	"ai.analysis_system_prompt": {Type: "string", Default: "", Encrypted: false},
	"ai.analysis_user_prefix":  {Type: "string", Default: "", Encrypted: false},
	"ai.speech.provider":       {Type: "string", Default: "none", Encrypted: false},
	"ai.speech.base_url":       {Type: "string", Default: "", Encrypted: false},
	"ai.speech.api_key":        {Type: "string", Default: "", Encrypted: true},
	"ai.speech.model":          {Type: "string", Default: "whisper-1", Encrypted: false},
	"ai.speech.language":       {Type: "string", Default: "zh", Encrypted: false},

	// Chevereto image hosting settings
	"chevereto.enabled":  {Type: "bool", Default: false, Encrypted: false},
	"chevereto.domain":   {Type: "string", Default: "", Encrypted: false},
	"chevereto.api_key":  {Type: "string", Default: "", Encrypted: true},
	"chevereto.album_id": {Type: "string", Default: "", Encrypted: false},

	// Image upload storage settings
	"image_upload.provider":            {Type: "string", Default: "local", Encrypted: false},
	"image_upload.local.path":          {Type: "string", Default: "", Encrypted: false},
	"image_upload.s3.bucket":           {Type: "string", Default: "", Encrypted: false},
	"image_upload.s3.region":           {Type: "string", Default: "", Encrypted: false},
	"image_upload.s3.endpoint":         {Type: "string", Default: "", Encrypted: false},
	"image_upload.s3.access_key":       {Type: "string", Default: "", Encrypted: true},
	"image_upload.s3.secret":           {Type: "string", Default: "", Encrypted: true},
	"image_upload.s3.force_path_style": {Type: "bool", Default: false, Encrypted: false},

	// Diary editor presets (deprecated - kept for backward compatibility)
	"diary.weather_options": {Type: "json", Default: []string{"☀️", "⛅", "☁️", "🌧️", "⛈️", "🌫️", "❄️", "🌬️"}, Encrypted: false},

	// Weather service settings
	"weather.enabled":  {Type: "bool", Default: false, Encrypted: false},
	"weather.mcp_url":  {Type: "string", Default: "http://localhost:8080", Encrypted: false},
	"weather.use_mcp":  {Type: "bool", Default: true, Encrypted: false},
	"weather.default_city": {Type: "string", Default: "", Encrypted: false},

	// Auto backup settings
	"backup.enabled":          {Type: "bool", Default: false, Encrypted: false},
	"backup.frequency":        {Type: "string", Default: "daily", Encrypted: false},
	"backup.time":             {Type: "string", Default: "00:00", Encrypted: false},
	"backup.day_of_week":      {Type: "int", Default: 1, Encrypted: false},
	"backup.day_of_month":     {Type: "int", Default: 1, Encrypted: false},
	"backup.retention_days":   {Type: "int", Default: 90, Encrypted: false},
	"backup.upload_s3":        {Type: "bool", Default: false, Encrypted: false},
}

// GetConfigMeta returns the metadata for a configuration key
func GetConfigMeta(key string) (ConfigMeta, bool) {
	meta, ok := ConfigRegistry[key]
	return meta, ok
}

// IsEncrypted checks if a configuration key should be encrypted
func IsEncrypted(key string) bool {
	if meta, ok := ConfigRegistry[key]; ok {
		return meta.Encrypted
	}
	return false
}

// GetDefault returns the default value for a configuration key
func GetDefault(key string) any {
	if meta, ok := ConfigRegistry[key]; ok {
		return meta.Default
	}
	return nil
}
