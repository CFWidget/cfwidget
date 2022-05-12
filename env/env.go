package env

import (
	"github.com/spf13/cast"
	"io"
	"log"
	"os"
	"strings"
)

var defaultEnvs = map[string]string{
	"DB_HOST":     "localhost",
	"DB_USER":     "widget",
	"DB_PASS":     "widget",
	"DB_DATABASE": "widget",
}

func init() {
	for k, v := range defaultEnvs {
		if Get(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

func Get(key string) string {
	filename := os.Getenv(key + "_FILE")
	if filename == "" {
		return os.Getenv(key)
	}
	val, err := readSecret(filename)
	if err != nil {
		log.Printf("error reading secret: %s", err.Error())
	}
	//cache value into global envs for re-use
	_ = os.Setenv(key, val)
	return val
}

func GetBool(key string) bool {
	return cast.ToBool(Get(key))
}

func GetInt(key string) int {
	return cast.ToInt(Get(key))
}

func readSecret(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}
