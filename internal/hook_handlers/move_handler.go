package hook_handlers

import (
	"context"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	appConfig "codiewuploader/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/tus/tusd/v2/pkg/hooks"
	"golang.org/x/exp/slog"
	"golang.org/x/image/draw"
)

const SwampDir = "rent_swamp"

type MoveHandler struct {
	config   appConfig.AppConfig
	s3Client *s3.Client
}

func NewMoveHandler(cfg appConfig.AppConfig) *MoveHandler {
	return &MoveHandler{
		config:   cfg,
		s3Client: InitS3Client(cfg),
	}
}

func InitS3Client(cfg appConfig.AppConfig) *s3.Client {
	s3Config, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	return s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.EndpointOptions.DisableHTTPS = false
		o.BaseEndpoint = &cfg.S3Endpoint
		o.UsePathStyle = true
	})
}

func (g *MoveHandler) Setup() error {
	log.Println("MoveHandler.Setup setup")
	return nil
}

func (g *MoveHandler) InvokeHook(req hooks.HookRequest) (res hooks.HookResponse, err error) {
	if req.Type != hooks.HookPostFinish {
		return res, nil
	}

	recordType, ok := req.Event.Upload.MetaData["recordType"]
	if !ok || recordType != "single" {
		slog.Info("Record not single", "id", req.Event.Upload.ID, "recordType", recordType, "metadata", req.Event.Upload.MetaData)

		return res, nil
	}

	entityId, ok := req.Event.Upload.MetaData["id"]
	if !ok {
		slog.Info("Record hasn't entityId in meta", "entityId", entityId)
		return res, nil
	}

	filename, ok := req.Event.Upload.MetaData["filename"]
	if !ok {
		slog.Info("Record hasn't entityId in meta", "filename", filename)
		return res, nil
	}

	contentType, ok := req.Event.Upload.MetaData["filetype"]
	if !ok {
		contentType = ""
	}

	id := req.Event.Upload.ID
	uploadId, _ := splitIds(id)

	slog.Info(
		"debug move info",
		"filename", filename,
		"contentType", contentType,
		"entityId", entityId,
		"uploadId", uploadId,
	)

	err = g.move(context.Background(), uploadId, entityId, filename, contentType)

	if err != nil {
		slog.Error("Move failed", "err", err.Error())
		return res, nil
	}

	return res, nil
}

/*
Перемещаем все наши записи в /{id}/... файлы записями
*/
func (g *MoveHandler) move(ctx context.Context, uploadId, entityId, filename, contentType string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpeg":
		ext = ".jpg"
	case ".png", ".jpg":
	default:
		ext = ".jpg"
	}

	res, _ := g.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(SwampDir),
		Key:    aws.String(uploadId),
	})

	tmpFile, err := ioutil.TempFile("", "tusd-s3-concat-tmp-")
	if err != nil {
		return err
	}
	defer cleanUpTempFile(tmpFile)

	if _, err := io.Copy(tmpFile, res.Body); err != nil {
		return err
	}

	_, err = tmpFile.Seek(0, 0)
	if err != nil {
		return err
	}

	outputFile, err := ioutil.TempFile("", fmt.Sprintf("tusd-s3-watermarked-*%s", ext))
	if err != nil {
		return err
	}
	outputFileName := outputFile.Name()
	_ = outputFile.Close()
	defer cleanUpTempFile(outputFile)

	cmd := exec.Command("ffmpeg",
		"-i", tmpFile.Name(),
		"-i", "/usr/local/share/watermark.png",
		"-filter_complex", "[1]scale=w=iw:h=-1[wm];[0][wm]overlay=0:0",
		"-y", outputFileName,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}

	finalFile, err := os.Open(outputFileName)
	if err != nil {
		return err
	}
	defer func(finalFile *os.File) {
		_ = finalFile.Close()
	}(finalFile)

	params := &s3.PutObjectInput{
		Bucket:      aws.String(g.config.ResultBucket),
		Key:         aws.String(fmt.Sprintf("%s/%s", entityId, filename)),
		Body:        finalFile,
		ACL:         types.ObjectCannedACLPublicRead,
		ContentType: aws.String(contentType),
	}
	if contentType != "" {
		params.ContentType = aws.String(contentType)
	}

	_, err = g.s3Client.PutObject(ctx, params)
	if err != nil {
		return err
	}

	// TODO:: (STEP_2) удалить файл и чанки и инфо, все старые файлы так как перемистили все, (вместе с шагом (STEP_1))

	return nil
}

func (g *MoveHandler) deleteExists(ctx context.Context, userID, replace string) {
	_, err := g.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(g.config.ResultBucket),
		Key:    aws.String(fmt.Sprintf("%s/%s", userID, replace)),
	})

	if err != nil {
		slog.Warn("Exists err", "filename", fmt.Sprintf("%s/%s", userID, replace))
		return
	}
}

func cleanUpTempFile(file *os.File) {
	file.Close()
	os.Remove(file.Name())
}

func splitIds(id string) (uploadId, multipartId string) {
	index := strings.Index(id, "+")
	if index == -1 {
		return
	}

	uploadId = id[:index]
	multipartId = id[index+1:]
	return
}

// applyOrientation корректирует изображение по EXIF ориентации
func applyOrientation(src image.Image, orientation int) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	switch orientation {
	case 2: // зеркально по горизонтали
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				dst.Set(bounds.Max.X-x-1, y, src.At(x, y))
			}
		}
	case 3: // перевернуть 180
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				dst.Set(bounds.Max.X-x-1, bounds.Max.Y-y-1, src.At(x, y))
			}
		}
	case 4: // зеркально по вертикали
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				dst.Set(x, bounds.Max.Y-y-1, src.At(x, y))
			}
		}
	case 5: // зеркально + поворот 90
		dst = rotate90(src, true)
	case 6: // поворот 90
		dst = rotate90(src, false)
	case 7: // зеркально + поворот 270
		dst = rotate270(src, true)
	case 8: // поворот 270
		dst = rotate270(src, false)
	default: // ориентация 1 — без изменений
		draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	}

	return dst
}

func rotate90(src image.Image, flip bool) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			px := src.At(x, y)
			if flip {
				dst.Set(y, b.Max.X-x-1, px)
			} else {
				dst.Set(y, x, px)
			}
		}
	}
	return dst
}

func rotate270(src image.Image, flip bool) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			px := src.At(x, y)
			if flip {
				dst.Set(b.Dy()-y-1, b.Max.X-x-1, px)
			} else {
				dst.Set(b.Dy()-y-1, x, px)
			}
		}
	}
	return dst
}
