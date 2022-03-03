package main

import (
	"encoding/json"
	"fmt"
	"github.com/lordralex/cfwidget/curseforge"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

var client = &http.Client{}

var gameCache = make(map[uint]curseforge.Game)
var categoryCache = make(map[uint][]curseforge.Category)

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

func updateGameCache() {
	response, err := callCurseForgeAPI("https://api.curseforge.com/v1/games")
	if err != nil {
		log.Printf("Error syncing game cache: %s", err.Error())
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := io.ReadAll(response.Body)
		log.Printf("Error from CurseForge for getting game cache: %s (%d)", string(body), response.StatusCode)
	}

	var data curseforge.GameResponse
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		log.Printf("Error parsing game cache: %s", err.Error())
		return
	}

	newMap := make(map[uint]curseforge.Game)
	for _, v := range data.Data {
		newMap[v.Id] = v
	}
	gameCache = newMap
}

func getCategories(gameId uint) []curseforge.Category {
	if gameId == 0 {
		return make([]curseforge.Category, 0)
	}

	if categories, exists := categoryCache[gameId]; exists {
		return categories
	}

	var data curseforge.CategoryResponse
	response, err := callCurseForgeAPI(fmt.Sprintf("https://api.curseforge.com/v1/categories?gameId=%d&pageSize=1000", gameId))
	if err != nil {
		log.Printf("Error getting categories for %d: %s", gameId, err.Error())
		return make([]curseforge.Category, 0)
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		log.Printf("Error getting categories for %d: %s", gameId, err.Error())
		return make([]curseforge.Category, 0)
	}
	categoryCache[gameId] = data.Data
	return data.Data
}

func getPrimaryCategoryFor(categories []curseforge.Category, id uint) curseforge.Category {
	for _, v := range categories {
		if v.Id == id {
			//if this is the highest, this is what we want
			if v.ParentCategoryId == 0 {
				return v
			}

			//otherwise... we need to see the parent of this one
			return getPrimaryCategoryFor(categories, v.ParentCategoryId)
		}
	}

	return curseforge.Category{}
}

func getGameBySlug(slug string) curseforge.Game {
	for _, v := range gameCache {
		if v.Slug == slug {
			return v
		}
	}

	return curseforge.Game{}
}