package hook_handlers

import (
	"log"

	appconfig "codiewuploader/internal/config"

	"github.com/tus/tusd/v2/pkg/hooks"
)

type FfmpegConvertHandler struct {
	config appconfig.AppConfig
}

func NewFfmpegConvertHandler(config appconfig.AppConfig) *FfmpegConvertHandler {
	return &FfmpegConvertHandler{
		config: config,
	}
}

func (g *FfmpegConvertHandler) Setup() error {
	log.Println("FfmpegConvertHandler.Setup setup")
	return nil
}

// eyJhdWQiOiJhdXRoZW50aWNhdGVkIiwiZXhwIjoxNzI5MTc3MDUxLCJpYXQiOjE3MjkxNzM0NTEsImlzcyI6Imh0dHA6Ly8xMjcuMC4wLjE6NTQzMjEvYXV0aC92MSIsInN1YiI6ImI5YTVmNzJiLWIzZGUtNDIyNy05ZjZlLTdjNGU3NTMxOWZlMSIsImVtYWlsIjoibWZlZG9yb3ZAZ2FyYS5nZSIsInBob25lIjoiIiwiYXBwX21ldGFkYXRhIjp7InByb3ZpZGVyIjoiZ29vZ2xlIiwicHJvdmlkZXJzIjpbImdvb2dsZSJdfSwidXNlcl9tZXRhZGF0YSI6eyJhdmF0YXJfdXJsIjoiaHR0cHM6Ly9saDMuZ29vZ2xldXNlcmNvbnRlbnQuY29tL2EvQUNnOG9jSmpPbmJKaFJxN3hCbDM5VjVNNWNpN2hVb2g0TzRwYnoxUDRKQWVjWW55US04V1JWQT1zOTYtYyIsImN1c3RvbV9jbGFpbXMiOnsiaGQiOiJnYXJhLmdlIn0sImVtYWlsIjoibWZlZG9yb3ZAZ2FyYS5nZSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlLCJmdWxsX25hbWUiOiJNYWtzaW0gRmVkb3JvdiIsImlzcyI6Imh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbSIsIm5hbWUiOiJNYWtzaW0gRmVkb3JvdiIsInBob25lX3ZlcmlmaWVkIjpmYWxzZSwicGljdHVyZSI6Imh0dHBzOi8vbGgzLmdvb2dsZXVzZXJjb250ZW50LmNvbS9hL0FDZzhvY0pqT25iSmhScTd4QmwzOVY1TTVjaTdoVW9oNE80cGJ6MVA0SkFlY1lueVEtOFdSVkE9czk2LWMiLCJwcm92aWRlcl9pZCI6IjExNzc1NDM5NDAwOTUyMTI5NzcyNSIsInN1YiI6IjExNzc1NDM5NDAwOTUyMTI5NzcyNSJ9LCJyb2xlIjoiYXV0aGVudGljYXRlZCIsImFhbCI6ImFhbDEiLCJhbXIiOlt7Im1ldGhvZCI6Im9hdXRoIiwidGltZXN0YW1wIjoxNzI5MTY1NzkwfV0sInNlc3Npb25faWQiOiI3NGRkNWYzNi1kMjRkLTRiMTEtYjg0My02Y2I5NGQwZmUxNTkiLCJpc19hbm9ueW1vdXMiOmZhbHNlfQ
func (g *FfmpegConvertHandler) InvokeHook(req hooks.HookRequest) (res hooks.HookResponse, err error) {

	//err = ffmpeg.Input("./sample_data/in1.mp4").
	//	Output("./sample_data/out1.mp4", ffmpeg.KwArgs{"c:v": "libx265"}).
	//	OverWriteOutput().ErrorToStdOut().Run()
	//
	//if err != nil {
	//	g.errorResponse(&res)
	//	return res, nil
	//}

	return hooks.HookResponse{}, nil
}

func (g *FfmpegConvertHandler) errorResponse(res *hooks.HookResponse) {
	res.HTTPResponse.StatusCode = 401
	res.HTTPResponse.Body = ErrInvalidToken
	res.RejectUpload = true
}
