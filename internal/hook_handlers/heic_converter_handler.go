package hook_handlers

import (
	"log"

	appConfig "codiewuploader/internal/config"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tus/tusd/v2/pkg/hooks"
)

type HeicConverterHandler struct {
	config   appConfig.AppConfig
	s3Client *s3.Client
}

func NewHeicConverterHandler(cfg appConfig.AppConfig) *HeicConverterHandler {
	return &HeicConverterHandler{
		config:   cfg,
		s3Client: InitS3Client(cfg),
	}
}

func (g *HeicConverterHandler) Setup() error {
	log.Println("HeicConverterHandler.Setup setup")
	return nil
}

func (g *HeicConverterHandler) InvokeHook(req hooks.HookRequest) (res hooks.HookResponse, err error) {
	if req.Type != hooks.HookPostFinish {
		return res, nil
	}

	return res, nil
}
