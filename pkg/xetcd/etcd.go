package xetcd

type Config struct {
	Hosts []string
	Key   string
}

func NewConfig(hosts []string, key string) Config {
	return Config{Hosts: hosts, Key: key}
}
