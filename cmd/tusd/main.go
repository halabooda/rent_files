package main

import (
	"codiewuploader/cmd/tusd/cli"
	"codiewuploader/internal/composer"
	appConfig "codiewuploader/internal/config"
)

func main() {
	cli.PrepareRequiredEnvVars()
	cli.ParseFlags()
	cli.PrepareGreeting()

	// Print version and other information and exit if the -version flag has been
	// passed else we will start the HTTP server
	if cli.Flags.ShowVersion {
		cli.ShowVersion()
	} else {
		composer.CreateComposer(appConfig.S3ClientConfig{
			S3Bucket:                     cli.Flags.S3Bucket,
			S3ObjectPrefix:               cli.Flags.S3ObjectPrefix,
			S3Endpoint:                   cli.Flags.S3Endpoint,
			S3PartSize:                   cli.Flags.S3PartSize,
			S3MaxBufferedParts:           cli.Flags.S3MaxBufferedParts,
			S3DisableContentHashes:       cli.Flags.S3DisableContentHashes,
			S3DisableSSL:                 cli.Flags.S3DisableSSL,
			S3ConcurrentPartUploads:      cli.Flags.S3ConcurrentPartUploads,
			UploadDir:                    cli.Flags.UploadDir,
			MaxSize:                      cli.Flags.MaxSize,
			FilelockHolderPollInterval:   cli.Flags.FilelockHolderPollInterval,
			FilelockAcquirerPollInterval: cli.Flags.FilelockAcquirerPollInterval,
		})
		cli.Serve()
	}
}
