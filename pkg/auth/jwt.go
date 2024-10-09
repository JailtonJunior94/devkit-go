package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jailtonjunior94/financial/configs"

	"github.com/JailtonJunior94/devkit-go/pkg/o11y"

	"github.com/golang-jwt/jwt"
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrGenerateToken = errors.New("error trying to generate token")
)

type (
	JwtAdapter interface {
		GenerateToken(ctx context.Context, id, email string) (string, error)
		ValidateToken(ctx context.Context, tokenRequest string) (*User, error)
	}

	jwtAdapter struct {
		config *configs.Config
		o11y   o11y.Observability
	}

	User struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
)

func NewUser(id, email string) *User {
	return &User{ID: id, Email: email}
}

func NewJwtAdapter(config *configs.Config, o11y o11y.Observability) JwtAdapter {
	return &jwtAdapter{config: config, o11y: o11y}
}

func (j *jwtAdapter) GenerateToken(ctx context.Context, id, email string) (string, error) {
	_, span := j.o11y.Start(ctx, "jwt_adapter.generate_token")
	defer span.End()

	claims := jwt.MapClaims{
		"sub":   id,
		"email": email,
		"exp":   time.Now().Add(time.Hour * time.Duration(j.config.AuthExpirationAt)).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenSigned, err := token.SignedString([]byte(j.config.AuthSecretKey))
	if err != nil {
		span.AddAttributes(ctx, o11y.Error, "error trying to generate token",
			o11y.Attributes{Key: "e-mail", Value: email},
			o11y.Attributes{Key: "error", Value: err.Error()},
		)
		return "", ErrGenerateToken
	}
	return tokenSigned, nil
}

func (j *jwtAdapter) ValidateToken(ctx context.Context, tokenRequest string) (*User, error) {
	_, span := j.o11y.Start(ctx, "jwt_adapter.validate_token")
	defer span.End()

	tokenString := j.removeBearerPrefix(tokenRequest)
	if tokenString == "" {
		return nil, ErrInvalidToken
	}

	secret := []byte(j.config.AuthSecretKey)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			span.AddAttributes(ctx, o11y.Error, fmt.Sprintf("unexpected signing method: %v", token.Header["alg"]))
			return nil, ErrInvalidToken
		}
		return secret, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	user := NewUser(claims["sub"].(string), claims["email"].(string))
	return user, nil
}

func (j *jwtAdapter) removeBearerPrefix(tokenString string) string {
	if len(tokenString) > 7 && strings.ToUpper(tokenString[0:6]) == "BEARER" {
		return tokenString[7:]
	}
	return ""
}
