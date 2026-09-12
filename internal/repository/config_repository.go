package repository

// GitSync describes the machine-local half of the sync setup. It deliberately
// holds no credentials: a token is only ever read from the environment.
type GitSync struct {
	Enabled    bool   `json:"enabled"`
	Remote     string `json:"remote,omitempty"` // defaults to "origin"
	Branch     string `json:"branch,omitempty"` // defaults to "main"
	AutoPush   bool   `json:"auto_push"`
	DebounceMs int    `json:"debounce_ms,omitempty"` // defaults to 3000
	Auth       string `json:"auth,omitempty"`        // "ssh" or "token"; inferred from the URL when empty
	TokenUser  string `json:"token_user,omitempty"`  // "x-access-token" for GitHub, "oauth2" for GitLab
	TokenEnv   string `json:"token_env,omitempty"`   // defaults to "LAZYTODO_GIT_TOKEN"
}

type Config struct {
	// Vertical is the pre-v4 layout preference. It is read once to seed
	// Settings and is no longer written; see SettingsRepository.
	Vertical    bool     `json:"vertical"`
	StorageRoot string   `json:"storage_root,omitempty"`
	Git         *GitSync `json:"git,omitempty"`
}

type ConfigRepository interface {
	Load() (Config, error)
	Save(Config) error
}
