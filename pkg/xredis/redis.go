package xredis

import goredis "github.com/zeromicro/go-zero/core/stores/redis"

type (
	Config = goredis.RedisConf
	Client = goredis.Redis
)

func New(conf Config) (*Client, error) {
	return goredis.NewRedis(conf)
}

func MustNew(conf Config) *Client {
	return goredis.MustNewRedis(conf)
}
