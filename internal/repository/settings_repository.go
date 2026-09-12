package repository

// Settings holds preferences that live beside the task data rather than in the
// machine-local config, so they travel with it when the storage root is a git
// repository.
type Settings struct {
	// Vertical is nil until the user first sets a layout, which lets callers
	// distinguish "not chosen yet" from "explicitly horizontal" and seed the
	// value from the legacy config only once.
	Vertical *bool `json:"vertical,omitempty"`
}

type SettingsRepository interface {
	Load() (Settings, error)
	Save(Settings) error
}
