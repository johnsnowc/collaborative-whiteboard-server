package middleware

import (
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

type HttpMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// Auth middleware, check the token
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the token in the request context
		token := c.Request.Header.Get("token")
		if token == "" {
			c.JSON(http.StatusOK, HttpMessage{
				Code:    "-1",
				Message: "No token, illegal access",
				Data:    nil,
			})
			c.Abort()
			return
		}
		log.Print("get token:", token)
		jwt := NewJWT()
		claims, err := jwt.ParseToken(token)
		if err != nil {
			if err == TokenExpired {
				c.JSON(http.StatusOK, HttpMessage{
					Code:    "-1",
					Message: "Authorization expired",
					Data:    nil,
				})
				c.Abort()
				return
			}
			c.JSON(http.StatusOK, HttpMessage{
				Code:    "-1",
				Message: err.Error(),
				Data:    nil,
			})
			c.Abort()
			return
		}
		// Proceed to the next route and pass on the parsed information
		c.Set("username", claims.Name)
	}
}

// JWT signature structure
type JWT struct {
	SigningKey []byte
}

var (
	TokenExpired     = errors.New("Token is expired")
	TokenNotValidYet = errors.New("Token not active yet")
	TokenMalformed   = errors.New("That's not even a token")
	TokenInvalid     = errors.New("Couldn't handle this token:")
	SignKey          = "johnsnowc"
)

// Create a new JWT instance
func NewJWT() *JWT {
	return &JWT{
		[]byte(GetSignKey()),
	}
}

// Access to signKey
func GetSignKey() string {
	return SignKey
}

func SetSignKey(key string) string {
	SignKey = key
	return SignKey
}

// Payloads, you can add the information you need
type CustomClaims struct {
	Name string `json:"name"`
	jwt.StandardClaims
}

// CreateToken generates a token
func (j *JWT) CreateToken(claims CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.SigningKey)
}
func (j *JWT) ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.SigningKey, nil
	})
	// There is a problem with token resolution
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return nil, TokenMalformed
			} else if ve.Errors&jwt.ValidationErrorExpired != 0 {
				// Token is expired
				return nil, TokenExpired
			} else if ve.Errors&jwt.ValidationErrorNotValidYet != 0 {
				return nil, TokenNotValidYet
			} else {
				return nil, TokenInvalid
			}
		}
	}
	if token != nil {
		if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
			return claims, nil
		}
	}
	return nil, TokenInvalid
}
