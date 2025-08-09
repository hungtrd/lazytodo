package repository

type Config struct {
	Vertical bool `json:"vertical"`
}

type ConfigRepository interface {
	Load() (Config, error)
	Save(Config) error
}
