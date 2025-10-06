package hook_handlers

import (
	"fmt"
	"github.com/form3tech-oss/jwt-go"
	"github.com/tus/tusd/v2/pkg/hooks"
	"log"

	appconfig "codiewuploader/internal/config"
)

var ErrInvalidToken = "Invalid upload token"

type AuthHandler struct {
	config appconfig.AppConfig
}

func NewAuthHandler(config appconfig.AppConfig) *AuthHandler {
	return &AuthHandler{
		config: config,
	}
}

func (g *AuthHandler) Setup() error {
	log.Println("AuthHandler.Setup setup")
	return nil
}

const triggerTokenToken = "x1Zs5zYFWc1Xp1Rt7xFqP8eT6s"

// eyJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzI5MTc3MDUxLCJpYXQiOjE3MjkxNzM0NTEsImlzcyI6Imh0dHA6Ly8xMjcuMC4wLjE6NTQzMjEvYXV0aC92MSIsInN1YiI6ImI5YTVmNzJiLWIzZGUtNDIyNy05ZjZlLTdjNGU3NTMxOWZlMSIsImVtYWlsIjoibWZlZG9yb3ZAZ2FyYS5nZSIsInBob25lIjoiIiwiYXBwX21ldGFkYXRhIjp7InByb3ZpZGVyIjoiZ29vZ2xlIiwicHJvdmlkZXJzIjpbImdvb2dsZSJdfSwidXNlcl9tZXRhZGF0YSI6eyJhdmF0YXJfdXJsIjoiaHR0cHM6Ly9saDMuZ29vZ2xldXNlcmNvbnRlbnQuY29tL2EvQUNnOG9jSmpPbmJKaFJxN3hCbDM5VjVNNWNpN2hVb2g0TzRwYnoxUDRKQWVjWW55US04V1JWQT1zOTYtYyIsImN1c3RvbV9jbGFpbXMiOnsiaGQiOiJnYXJhLmdlIn0sImVtYWlsIjoibWZlZG9yb3ZAZ2FyYS5nZSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlLCJmdWxsX25hbWUiOiJNYWtzaW0gRmVkb3JvdiIsImlzcyI6Imh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbSIsIm5hbWUiOiJNYWtzaW0gRmVkb3JvdiIsInBob25lX3ZlcmlmaWVkIjpmYWxzZSwicGljdHVyZSI6Imh0dHBzOi8vbGgzLmdvb2dsZXVzZXJjb250ZW50LmNvbS9hL0FDZzhvY0pqT25iSmhScTd4QmwzOVY1TTVjaTdoVW9oNE80cGJ6MVA0SkFlY1lueVEtOFdSVkE9czk2LWMiLCJwcm92aWRlcl9pZCI6IjExNzc1NDM5NDAwOTUyMTI5NzcyNSIsInN1YiI6IjExNzc1NDM5NDAwOTUyMTI5NzcyNSJ9LCJyb2xlIjoiYXV0aGVudGljYXRlZCIsImFhbCI6ImFhbDEiLCJhbXIiOlt7Im1ldGhvZCI6Im9hdXRoIiwidGltZXN0YW1wIjoxNzI5MTY1NzkwfV0sInNlc3Npb25faWQiOiI3NGRkNWYzNi1kMjRkLTRiMTEtYjg0My02Y2I5NGQwZmUxNTkiLCJpc19hbm9ueW1vdXMiOmZhbHNlfQ
func (g *AuthHandler) InvokeHook(req hooks.HookRequest) (res hooks.HookResponse, err error) {
	if req.Type != hooks.HookPreCreate {
		return res, nil
	}

	uploadToken, ok2 := req.Event.HTTPRequest.Header["Upload-Token"]
	if !ok2 || len(uploadToken) < 1 {
		g.errorResponse(&res)
		return res, nil
	}

	token, err := parseToken(uploadToken[0], []byte(g.config.JwtSecret))
	if err != nil || token == nil {
		g.errorResponse(&res)
		return res, nil
	}

	claims, _ := token.Claims.(jwt.MapClaims)
	userId, ok := claims["sub"]
	if !ok || userId == "" {
		g.errorResponse(&res)
		return res, nil
	}

	return res, nil
}

func (g *AuthHandler) errorResponse(res *hooks.HookResponse) {
	res.HTTPResponse.StatusCode = 401
	res.HTTPResponse.Body = ErrInvalidToken
	res.RejectUpload = true
}

func parseToken(input string, secretKey []byte) (*jwt.Token, error) {
	token, _ := jwt.Parse(input, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	return token, nil
}
