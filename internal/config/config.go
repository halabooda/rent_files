package config

import "time"

type S3ClientConfig struct {
	S3Bucket                     string
	S3ObjectPrefix               string
	S3Endpoint                   string
	S3PartSize                   int64
	S3MaxBufferedParts           int64
	S3DisableContentHashes       bool
	S3DisableSSL                 bool
	S3ConcurrentPartUploads      int
	UploadDir                    string
	MaxSize                      int64
	FilelockHolderPollInterval   time.Duration
	FilelockAcquirerPollInterval time.Duration
}

type AppConfig struct {
	SupabaseApiURL        string
	SupabaseApiServiceKey string
	SupabaseJwtSecret     string
	S3Endpoint            string

	ResultBucket string
}
