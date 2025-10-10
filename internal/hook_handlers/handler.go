package hook_handlers

import (
	"fmt"
	"github.com/tus/tusd/v2/pkg/hooks"
	"os"

	appconfig "codiewuploader/internal/config"
)

type Handler struct {
	handlers []hooks.HookHandler
}

func NewHandler(s3Endpoint string) *Handler {
	config := appconfig.AppConfig{
		JwtSecret:    os.Getenv("JWT_SECRET"),
		ResultBucket: os.Getenv("RECORD_BUCKET"),

		S3Endpoint: s3Endpoint,
	}
	_ = config

	return &Handler{
		handlers: []hooks.HookHandler{
			NewAuthHandler(config),
			NewHeicConverterHandler(config),
			//NewFinishHandler(config),
			NewMoveHandler(config),
			NewFfmpegConvertHandler(config),
		},
	}
}

func (g *Handler) Setup() error {
	return nil
}

func (g *Handler) InvokeHook(req hooks.HookRequest) (res hooks.HookResponse, err error) {
	res.HTTPResponse.Header = make(map[string]string)

	// Sub handlers
	for _, handler := range g.handlers {
		subRes, subErr := handler.InvokeHook(req)
		if subErr != nil {
			return subRes, subErr
		}

		if subRes.RejectUpload {
			return subRes, subErr
		}

		if subRes.StopUpload {
			return subRes, subErr
		}

		for subHeaderKey, subHeaderValue := range subRes.HTTPResponse.Header {
			res.HTTPResponse.Header[subHeaderKey] = subHeaderValue
		}

		res.HTTPResponse.Body = fmt.Sprintf("%s, %s", res.HTTPResponse.Body, subRes.HTTPResponse.Body)

		// Берем больштй HTTP код от сабхэнделра, который определит, что ошибка выше
		if subRes.HTTPResponse.StatusCode > 399 {
			res.HTTPResponse.StatusCode = subRes.HTTPResponse.StatusCode
		}

		// Изменения меты: если совпал ключ, то перезаписываем (TODO:: может объединить?),
		// если нет — просто добавляем
		for subResMetaKey, subResMetaValue := range subRes.ChangeFileInfo.MetaData {
			res.ChangeFileInfo.MetaData[subResMetaKey] = subResMetaValue
		}
	}

	return res, nil
}
