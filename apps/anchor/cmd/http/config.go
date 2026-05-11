package httpserver

type ServerConfig struct {
	Port          int    `yaml:"port"`
	AllowedOrigin string `yaml:"allowed_origin"`
}
