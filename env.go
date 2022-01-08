package main

import "os"

var defaultEnvs = map[string]string{
	"DB_HOST":     "localhost",
	"DB_USER":     "widget",
	"DB_PASS":     "widget",
	"DB_DATABASE": "widget",
}

func init() {
	for k, v := range defaultEnvs {
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
