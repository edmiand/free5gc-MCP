package auth

import (
    "errors"
    "io/ioutil"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
    "github.com/gin-gonic/gin"
)

type AuthConfig struct {
    Type string // "static" | "jwt" | ""
    StaticToken string
    JWTSecret string
    JWTPublicKeyPath string

    // internal
    jwtKey interface{}
}

func (a *AuthConfig) IsEnabled() bool {
    return a != nil && (a.Type == "static" || a.Type == "jwt")
}

func (a *AuthConfig) Load() error {
    if a == nil {
        return nil
    }
    if a.Type == "jwt" {
        if a.JWTPublicKeyPath != "" {
            b, err := ioutil.ReadFile(a.JWTPublicKeyPath)
            if err != nil {
                return err
            }
            // try parse RSA public key
            key, err := jwt.ParseRSAPublicKeyFromPEM(b)
            if err != nil {
                return err
            }
            a.jwtKey = key
            return nil
        }
        if a.JWTSecret != "" {
            a.jwtKey = []byte(a.JWTSecret)
            return nil
        }
    }
    return nil
}

func (a *AuthConfig) Middleware() gin.HandlerFunc {
    if a == nil {
        return func(c *gin.Context) { c.Next() }
    }
    switch a.Type {
    case "static":
        return func(c *gin.Context) {
            hdr := c.GetHeader("Authorization")
            if hdr == "" || !strings.HasPrefix(hdr, "Bearer ") || hdr[len("Bearer "):] != a.StaticToken {
                c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
                return
            }
            c.Next()
        }
    case "jwt":
        return func(c *gin.Context) {
            hdr := c.GetHeader("Authorization")
            if hdr == "" || !strings.HasPrefix(hdr, "Bearer ") {
                c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
                return
            }
            tokenStr := hdr[len("Bearer "):]
            var keyFunc jwt.Keyfunc
            if a.jwtKey == nil {
                c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "jwt key not loaded"})
                return
            }
            switch k := a.jwtKey.(type) {
            case []byte:
                keyFunc = func(t *jwt.Token) (interface{}, error) {
                    // allow only HMAC
                    if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                        return nil, errors.New("unexpected signing method")
                    }
                    return k, nil
                }
            default:
                keyFunc = func(t *jwt.Token) (interface{}, error) {
                    // allow RSA
                    if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
                        return nil, errors.New("unexpected signing method")
                    }
                    return k, nil
                }
            }
            token, err := jwt.Parse(tokenStr, keyFunc)
            if err != nil || !token.Valid {
                c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid jwt"})
                return
            }
            c.Set("jwt", token)
            c.Next()
        }
    default:
        return func(c *gin.Context) { c.Next() }
    }
}
