package env

import (
	"github.com/spf13/cast"
	"io"
	"log"
	"os"
	"strings"
)

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

func GetOr(key string, def string) string {
	res := Get(key)
	if res == "" {
		return def
	}
	return res
}

func GetBool(key string) bool {
	return cast.ToBool(Get(key))
}

func GetBoolOr(key string, def bool) bool {
	res := Get(key)
	if res == "" {
		return def
	}
	return cast.ToBool(res)
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
