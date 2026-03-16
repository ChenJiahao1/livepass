package xmysql

type Config struct {
	DataSource string
}

func NewConfig(dataSource string) Config {
	return Config{DataSource: dataSource}
}
