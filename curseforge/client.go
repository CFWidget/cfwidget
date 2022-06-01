package curseforge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cfwidget/cfwidget/env"
	"go.elastic.co/apm/module/apmhttp/v2"
	"go.elastic.co/apm/v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

var client *http.Client

var gameCache = make(map[uint]Game)
var categoryCache = make(map[uint][]Category)

const PageSize = 50

func init() {
	client = apmhttp.WrapClient(&http.Client{})
}

func StartGameCacheSyncer() {
	go func() {
		err := updateGameCache()
		if err != nil {
			log.Printf("Error updating game cache: %s\n", err.Error())
		}

		ticker := time.NewTicker(time.Hour)
		for {
			select {
			case <-ticker.C:
				err = updateGameCache()
				if err != nil {
					log.Printf("Error updating game cache: %s\n", err.Error())
				}
			}
		}
	}()
}

func Call(u string, ctx context.Context) (*http.Response, error) {
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

	response, err := client.Do(request.WithContext(ctx))

	if env.GetBool("DEBUG") {
		//clone body so we can "replace" it
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		response.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		log.Printf("URL %s\nResult: %s\nBody: %s\n", path.String(), response.Status, string(body))
	}

	return response, err
}

func updateGameCache() error {
	trans := apm.DefaultTracer().StartTransaction("gameCacheSync", "schedule")
	defer trans.End()

	defer func() {
		err := recover()
		if err != nil {
			trans.Outcome = "failure"
		}
	}()

	ctx := apm.ContextWithTransaction(context.Background(), trans)

	games := make([]Game, 0)
	page := uint(0)

	for {
		response, err := getGames(page, ctx)
		if err != nil {
			trans.Outcome = "failure"
			return err
		}

		if len(response.Data) > 0 {
			games = append(games, response.Data...)
		}

		//if we don't have the same number as the page size, we have them all
		if response.Pagination.ResultCount < PageSize {
			break
		}

		page++
	}

	newMap := make(map[uint]Game)
	for _, v := range games {
		newMap[v.Id] = v
	}
	gameCache = newMap
	return nil
}

func GetGame(gameId uint) Game {
	return gameCache[gameId]
}

func GetCategories(gameId uint, ctx context.Context) ([]Category, error) {
	if gameId == 0 {
		return make([]Category, 0), nil
	}

	if categories, exists := categoryCache[gameId]; exists {
		return categories, nil
	}

	categories := make([]Category, 0)
	page := uint(0)

	for {
		response, err := getCategories(gameId, page, ctx)
		if err != nil {
			return categories, err
		}

		if len(response.Data) > 0 {
			categories = append(categories, response.Data...)
		}

		//if we don't have the same number as the page size, we have them all
		if response.Pagination.ResultCount < PageSize {
			break
		}

		page++
	}

	categoryCache[gameId] = categories
	return categories, nil
}

func GetPrimaryCategoryFor(categories []Category, id uint) Category {
	for _, v := range categories {
		if v.Id == id {
			//if this is the highest, this is what we want
			if v.ParentCategoryId == 0 {
				return v
			}

			//otherwise... we need to see the parent of this one
			return GetPrimaryCategoryFor(categories, v.ParentCategoryId)
		}
	}

	return Category{}
}

func GetGameBySlug(slug string) Game {
	for _, v := range gameCache {
		if v.Slug == slug {
			return v
		}
	}

	return Game{}
}

func getCategories(gameId, page uint, ctx context.Context) (CategoryResponse, error) {
	var data CategoryResponse
	response, err := Call(fmt.Sprintf("https://api.curseforge.com/v1/categories?gameId=%d&pageSize=%d&index=%d", gameId, PageSize, PageSize*page), ctx)
	if err != nil {
		return CategoryResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return CategoryResponse{}, errors.New(fmt.Sprintf("invalid status code: %s", response.Status))
	}

	err = json.NewDecoder(response.Body).Decode(&data)
	return data, err
}

func getGames(page uint, ctx context.Context) (GameResponse, error) {
	var data GameResponse
	response, err := Call(fmt.Sprintf("https://api.curseforge.com/v1/games?pageSize=%d&index=%d", PageSize, PageSize*page), ctx)
	if err != nil {
		return GameResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return GameResponse{}, errors.New(fmt.Sprintf("invalid status code: %s", response.Status))
	}

	err = json.NewDecoder(response.Body).Decode(&data)
	return data, err
}

func GetFiles(projectId uint, ctx context.Context) ([]File, error) {
	files := make([]File, 0)
	page := uint(0)

	for {
		response, err := getFilesForPage(projectId, page, ctx)
		if err != nil {
			return nil, err
		}

		if len(response.Data) > 0 {
			files = append(files, response.Data...)
		}

		//if we don't have the same number as the page size, we have them all
		if response.Pagination.ResultCount < PageSize {
			break
		}

		page++
	}

	return files, nil
}

func getFilesForPage(projectId, page uint, ctx context.Context) (FilesResponse, error) {
	u := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files?index=%d&pageSize=%d", projectId, page*PageSize, PageSize)

	response, err := Call(u, ctx)
	if err != nil {
		return FilesResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode == 404 {
		return FilesResponse{}, nil
	}

	if response.StatusCode != 200 {
		return FilesResponse{}, errors.New(fmt.Sprintf("invalid status code: %s", response.Status))
	}

	var files FilesResponse
	err = json.NewDecoder(response.Body).Decode(&files)
	return files, err
}
