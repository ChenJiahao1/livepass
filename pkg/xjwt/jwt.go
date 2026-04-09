package xjwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	AccessSecret string
	AccessExpire time.Duration
}

type Claims struct {
	UserID int64 `json:"userId"`
	jwt.RegisteredClaims
}

func ExpireAt(now time.Time, expire time.Duration) time.Time {
	return now.Add(expire)
}

func CreateToken(userID int64, secret string, expire time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(ExpireAt(time.Now(), expire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(tokenString, secret string) (*Claims, error) {
	claims := new(Claims)
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}
