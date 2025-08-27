package mystic

type Config struct {
	LogLevel string `env:"LOG_LEVEL, required"`
	Facility string `env:"FACILITY, required"`
}
