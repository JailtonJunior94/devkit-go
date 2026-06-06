package worker

import "time"

const defaultShutdownTimeout = 30 * time.Second

type Config struct {
	ShutdownTimeout time.Duration
}

func defaultConfig() Config {
	return Config{
		ShutdownTimeout: defaultShutdownTimeout,
	}
}
