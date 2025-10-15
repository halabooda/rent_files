package hook_handlers

import (
	"context"
	"fmt"
	"github.com/disintegration/imaging"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
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

	mediaType, ok := req.Event.Upload.MetaData["mediatype"]
	if !ok {
		slog.Info("Record hasn't entityId in meta", "mediatype", filename)
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
		"mediaType", mediaType,
	)

	err = g.move(context.Background(), uploadId, entityId, filename, contentType, mediaType)

	if err != nil {
		slog.Error("Move failed", "err", err.Error())
		return res, nil
	}

	return res, nil
}

/*
Перемещаем все наши записи в /{id}/... файлы записями
*/
func (g *MoveHandler) move(ctx context.Context, uploadId, entityId, filename, contentType, mediaType string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	if mediaType == "image" {
		switch ext {
		case ".jpeg":
			ext = ".jpg"
		case ".png", ".jpg":
		}
	}

	res, _ := g.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(SwampDir),
		Key:    aws.String(uploadId),
	})

	originalFile, err := ioutil.TempFile("", "tusd-s3-concat-tmp-")
	if err != nil {
		return err
	}
	defer cleanUpTempFile(originalFile)

	if _, err := io.Copy(originalFile, res.Body); err != nil {
		return err
	}

	_, err = originalFile.Seek(0, 0)
	if err != nil {
		return err
	}

	originalName := fmt.Sprintf("%s/%s", entityId, filename)
	// для картинок прячем названием за имя с id в название и промежуточным префиксом -original-
	if mediaType == "image" {
		originalName = fmt.Sprintf("%s/%s-original-%s", entityId, entityId, filename)
	}

	params := &s3.PutObjectInput{
		Bucket:      aws.String(g.config.ResultBucket),
		Key:         aws.String(originalName),
		Body:        originalFile,
		ACL:         types.ObjectCannedACLPublicRead,
		ContentType: aws.String(contentType),
	}

	_, err = g.s3Client.PutObject(ctx, params)
	if err != nil {
		return err
	}

	if _, err := originalFile.Seek(0, 0); err != nil {
		return err
	}

	if mediaType == "image" {
		// === ЛОГИКА ВОДЯНОГО ЗНАКА ===
		img, _, err := image.Decode(originalFile)
		if err != nil {
			return err
		}

		watermarkFile, err := os.Open("/usr/local/share/watermark60.png")
		if err != nil {
			return err
		}
		defer watermarkFile.Close()

		watermark, _, err := image.Decode(watermarkFile)
		if err != nil {
			return err
		}

		wWidth := img.Bounds().Dx()
		scale := float64(wWidth) / float64(watermark.Bounds().Dx())
		wHeight := int(float64(watermark.Bounds().Dy()) * scale)
		resizedWatermark := imaging.Resize(watermark, wWidth, wHeight, imaging.Lanczos)

		result := imaging.Clone(img)

		// Вычисляем координаты центра
		x := 0 // по ширине мы масштабировали водяной знак на всю ширину
		y := (img.Bounds().Dy() - wHeight) / 2

		draw.Draw(result, image.Rect(x, y, x+wWidth, y+wHeight), resizedWatermark, image.Point{}, draw.Over)

		if err := originalFile.Truncate(0); err != nil {
			return err
		}
		if _, err := originalFile.Seek(0, 0); err != nil {
			return err
		}

		if ext == ".png" {
			if err := png.Encode(originalFile, result); err != nil {
				return err
			}
			contentType = "image/png"
		} else {
			if err := jpeg.Encode(originalFile, result, &jpeg.Options{Quality: 90}); err != nil {
				return err
			}
			contentType = "image/jpeg"
		}

		if _, err := originalFile.Seek(0, 0); err != nil {
			return err
		}
		// === КОНЕЦ ЛОГИКИ ВОДЯНОГО ЗНАКА ===

		params := &s3.PutObjectInput{
			Bucket:      aws.String(g.config.ResultBucket),
			Key:         aws.String(fmt.Sprintf("%s/%s", entityId, filename)),
			Body:        originalFile,
			ACL:         types.ObjectCannedACLPublicRead,
			ContentType: aws.String(contentType),
		}

		_, err = g.s3Client.PutObject(ctx, params)
		if err != nil {
			return err
		}
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
