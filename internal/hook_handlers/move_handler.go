package hook_handlers

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/tus/tusd/v2/pkg/hooks"
	"golang.org/x/exp/slog"
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

	recordID, ok := req.Event.Upload.MetaData["id"]
	if !ok {
		slog.Info("Record hasn't recordID in meta", "recordID", recordID)
		return res, nil
	}

	filename, ok := req.Event.Upload.MetaData["filename"]
	if !ok {
		slog.Info("Record hasn't recordID in meta", "filename", filename)
		return res, nil
	}

	contentType, ok := req.Event.Upload.MetaData["filetype"]
	if !ok {
		contentType = ""
	}

	replace, ok := req.Event.Upload.MetaData["replace"]
	if !ok {
		replace = ""
	}

	id := req.Event.Upload.ID
	uploadId, _ := splitIds(id)

	err = g.move(context.Background(), uploadId, recordID, filename, contentType, replace)

	if err != nil {
		slog.Error("Move failed", "err", err.Error())
		return res, nil
	}

	return res, nil
}

/*
Перемещаем все наши записи в /{recordId}/... файлы записями
*/
func (g *MoveHandler) move(ctx context.Context, uploadId, recordID, filename, contentType string, replace string) error {
	res, _ := g.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(SwampDir),
		Key:    aws.String(uploadId),
	})

	file, err := ioutil.TempFile("", "tusd-s3-concat-tmp-")
	if err != nil {
		return err
	}
	defer cleanUpTempFile(file)

	if _, err := io.Copy(file, res.Body); err != nil {
		return err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}

	if replace != "" {
		g.deleteExists(ctx, recordID, replace)
	}

	params := &s3.PutObjectInput{
		Bucket:      aws.String(g.config.ResultBucket),
		Key:         aws.String(fmt.Sprintf("%s/%s", recordID, filename)),
		Body:        file,
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

func (g *MoveHandler) deleteExists(ctx context.Context, recordID, replace string) {
	_, err := g.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(g.config.ResultBucket),
		Key:    aws.String(fmt.Sprintf("%s/%s", recordID, replace)),
	})

	if err != nil {
		slog.Warn("Exists err", "filename", fmt.Sprintf("%s/%s", recordID, replace))
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

func jsonBinaryToString(reader io.ReadCloser) (*string, interface{}, error) {
	// Чтение данных из ReadCloser
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, err
	}

	// Закрытие ReadCloser
	defer reader.Close()

	// Преобразование JSON в map[string]interface{} (если структура неизвестна)
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, nil, err
	}

	// Преобразование обратно в строку JSON (если это необходимо)
	jsonString, err := json.Marshal(result)
	if err != nil {
		fmt.Println("Error marshaling:", err)
		return nil, nil, err
	}

	res := string(jsonString)

	return &res, result, nil
}
