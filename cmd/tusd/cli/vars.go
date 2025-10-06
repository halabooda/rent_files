package cli

import (
	"codiewuploader/internal/log"
	"os"
)

var envs = []string{
	"AWS_REGION",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	//"SUPABASE_API_URL",
	//"SUPABASE_API_SERVICEKEY",
	//"SUPABASE_JWT_SECRET",
}

func PrepareRequiredEnvVars() {
	for _, env := range envs {
		val := os.Getenv(env)
		if val == "" {
			log.Stderr.Fatalf("`%s` env var must be defined", env)
		}
	}
}
