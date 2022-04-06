package curseforge

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

var client = &http.Client{}

var gameCache = make(map[uint]Game)
var categoryCache = make(map[uint][]Category)

const PageSize = 50

func init() {
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

func Call(u string) (*http.Response, error) {
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

func updateGameCache() error {
	games := make([]Game, 0)
	page := uint(0)

	for {
		response, err := getGames(page)
		if err != nil {
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

func GetCategories(gameId uint) ([]Category, error) {
	if gameId == 0 {
		return make([]Category, 0), nil
	}

	if categories, exists := categoryCache[gameId]; exists {
		return categories, nil
	}

	categories := make([]Category, 0)
	page := uint(0)

	for {
		response, err := getCategories(gameId, page)
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

func getCategories(gameId, page uint) (CategoryResponse, error) {
	var data CategoryResponse
	response, err := Call(fmt.Sprintf("https://api.curseforge.com/v1/categories?gameId=%d&pageSize=%d&index=%d", gameId, PageSize, PageSize*page))
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

func getGames(page uint) (GameResponse, error) {
	var data GameResponse
	response, err := Call(fmt.Sprintf("https://api.curseforge.com/v1/games?pageSize=%d&index=%d", PageSize, PageSize*page))
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

func GetFiles(projectId uint) ([]File, error) {
	files := make([]File, 0)
	page := uint(0)

	for {
		response, err := getFilesForPage(projectId, page)
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

func getFilesForPage(projectId, page uint) (FilesResponse, error) {
	u := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/files?index=%d&pageSize=%d", projectId, page*PageSize, PageSize)

	response, err := Call(u)
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
