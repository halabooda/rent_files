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
