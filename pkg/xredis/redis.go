package xredis

type Config struct {
	Host string
	Type string
	Pass string
}

func NewConfig(host string) Config {
	return Config{Host: host}
}
