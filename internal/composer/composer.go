package composer

import (
	"context"
	"log"
	"os"
	"path/filepath"

	appConfig "codiewuploader/internal/config"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	"github.com/tus/tusd/v2/pkg/handler"
	"github.com/tus/tusd/v2/pkg/memorylocker"
	"github.com/tus/tusd/v2/pkg/s3store"
)

var Stdout = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
var Stderr = log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)

var Composer *handler.StoreComposer

func CreateComposer(cfg appConfig.S3ClientConfig) {
	// Attempt to use S3 as a backend if the -s3-bucket option has been supplied.
	// If not, we default to storing them locally on disk.
	Composer = handler.NewStoreComposer()

	if cfg.S3Bucket != "" {
		// Derive credentials from default credential chain (env, shared, ec2 instance role)
		// as per https://github.com/aws/aws-sdk-go#configuring-credentials
		s3Config, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			Stderr.Fatalf("Unable to load S3 configuration: %s", err)
		}

		if cfg.S3Endpoint == "" {
			Stdout.Printf("Using 's3://%s' as S3 bucket for storage.\n", cfg.S3Bucket)
		} else {
			Stdout.Printf("Using '%s/%s' as S3 endpoint and bucket for storage.\n", cfg.S3Endpoint, cfg.S3Bucket)
		}

		s3Client := s3.NewFromConfig(s3Config, func(o *s3.Options) {
			o.UseAccelerate = false

			// Disable HTTPS and only use HTTP (helpful for debugging requests).
			o.EndpointOptions.DisableHTTPS = cfg.S3DisableSSL

			if cfg.S3Endpoint != "" {
				o.BaseEndpoint = &cfg.S3Endpoint
				o.UsePathStyle = true
			}
		})

		store := s3store.New(cfg.S3Bucket, s3Client)
		store.ObjectPrefix = cfg.S3ObjectPrefix
		store.PreferredPartSize = cfg.S3PartSize
		store.MaxBufferedParts = cfg.S3MaxBufferedParts
		store.DisableContentHashes = cfg.S3DisableContentHashes
		store.SetConcurrentPartUploads(cfg.S3ConcurrentPartUploads)
		store.UseIn(Composer)

		locker := memorylocker.New()
		locker.UseIn(Composer)

		// Attach the metrics from S3 store to the global Prometheus registry
		store.RegisterMetrics(prometheus.DefaultRegisterer)
	} else {
		dir, err := filepath.Abs(cfg.UploadDir)
		if err != nil {
			Stderr.Fatalf("Unable to make absolute path: %s", err)
		}

		Stdout.Printf("Using '%s' as directory storage.\n", dir)
		if err := os.MkdirAll(dir, os.FileMode(0774)); err != nil {
			Stderr.Fatalf("Unable to ensure directory exists: %s", err)
		}

		store := filestore.New(dir)
		store.UseIn(Composer)

		locker := filelocker.New(dir)
		locker.AcquirerPollInterval = cfg.FilelockAcquirerPollInterval
		locker.HolderPollInterval = cfg.FilelockHolderPollInterval
		locker.UseIn(Composer)
	}

	Stdout.Printf("Using %.2fMB as maximum size.\n", float64(cfg.MaxSize)/1024/1024)
}
