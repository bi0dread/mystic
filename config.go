package mystic

type Config struct {
	LogLevel    string `env:"LOG_LEVEL, required"`
	GrayLogAddr string `env:"GRAYLOG_ADDR"`
	Facility    string `env:"FACILITY, required"`
}
