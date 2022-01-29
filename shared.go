package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
)

var client = &http.Client{}

func coalesce(options ...string) string {
	for _, v := range options {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstOrEmpty(data []string) string {
	return firstOr(data, "")
}

func firstOr(data []string, def string) string {
	if len(data) == 0 {
		return def
	}
	return data[0]
}

func callCurseForgeAPI(u string) (*http.Response, error) {
	key := os.Getenv("CORE_KEY")

	path, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	request := &http.Request{
		Method: "GET",
		URL:    path,
		Header: http.Header{},
	}
	request.Header.Add("x-api-key", key)

	if os.Getenv("DEBUG") == "true" {
		log.Printf("Calling %s\n", path.String())
	}

	return client.Do(request)
}
