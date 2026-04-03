package store

// DBConfig holds database tuning parameters, read from the project config file.
type DBConfig struct {
	BusyTimeout int `yaml:"busy_timeout,omitempty" validate:"omitempty,gte=1"`
	MMapSize    int `yaml:"mmap_size,omitempty" validate:"omitempty,gte=0"`
	CacheSize   int `yaml:"cache_size,omitempty" validate:"omitempty,gte=0"`
}
