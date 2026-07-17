package repository

type Config struct {
	Vertical    bool   `json:"vertical"`
	StorageRoot string `json:"storage_root,omitempty"`
}

type ConfigRepository interface {
	Load() (Config, error)
	Save(Config) error
}
