package hook_handlers

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	appConfig "codiewuploader/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jdeng/goheif"
	"github.com/rwcarlsen/goexif/exif"
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

	isHeicString, ok := req.Event.Upload.MetaData["isHeicString"]
	if !ok {
		isHeicString = "false"
	}

	isHeic := false
	if isHeicString == "true" {
		isHeic = true
	}

	id := req.Event.Upload.ID
	uploadId, _ := splitIds(id)

	slog.Info(
		"debug move info",
		"filename", filename,
		"contentType", contentType,
		"entityId", entityId,
		"uploadId", uploadId,
		"isHeicString", isHeicString,
		"isHeic", isHeic,
	)

	err = g.move(context.Background(), uploadId, entityId, filename, contentType, isHeic)

	if err != nil {
		slog.Error("Move failed", "err", err.Error())
		return res, nil
	}

	return res, nil
}

/*
Перемещаем все наши записи в /{id}/... файлы записями
*/
func (g *MoveHandler) move(ctx context.Context, uploadId, entityId, filename, contentType string, isHeic bool) error {
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

	if strings.HasSuffix(strings.ToLower(filename), ".heic") {
		//if isHeic {
		img, err := goheif.Decode(tmpFile)
		if err != nil {
			return fmt.Errorf("decode HEIC failed: %w", err)
		}

		// Читаем EXIF ориентацию
		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
		exifData, _ := exif.Decode(tmpFile)
		orientation := 1
		if exifData != nil {
			if tag, err := exifData.Get(exif.Orientation); err == nil {
				orientation, _ = tag.Int(0)
			}
		}

		// Применяем ориентацию
		img = applyOrientation(img, orientation)

		// Создаём новый временный файл для JPEG
		jpegFile, err := ioutil.TempFile("", "tusd-s3-concat-jpeg-")
		if err != nil {
			return err
		}
		defer cleanUpTempFile(jpegFile)

		if err := jpeg.Encode(jpegFile, img, &jpeg.Options{Quality: 90}); err != nil {
			return err
		}

		if _, err := jpegFile.Seek(0, io.SeekStart); err != nil {
			return err
		}

		tmpFile = jpegFile
		contentType = "image/jpeg"
		filename = strings.TrimSuffix(filename, ".heic") + ".jpg"
	} else {
		if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	params := &s3.PutObjectInput{
		Bucket:      aws.String(g.config.ResultBucket),
		Key:         aws.String(fmt.Sprintf("%s/%s", entityId, filename)),
		Body:        tmpFile,
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
