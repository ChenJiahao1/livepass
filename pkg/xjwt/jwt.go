package xjwt

import "time"

type Config struct {
	AccessSecret string
	AccessExpire time.Duration
}

func ExpireAt(now time.Time, expire time.Duration) time.Time {
	return now.Add(expire)
}
