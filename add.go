package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lordralex/cfwidget/curseforge"
	"github.com/lordralex/cfwidget/env"
	"github.com/lordralex/cfwidget/widget"
	"github.com/spf13/cast"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var addProjectConsumer AddProjectConsumer
var FullPathWithId = regexp.MustCompile("[a-zA-Z\\-]+/[a-zA-Z\\-]+/([0-9]+)")

type AddProjectConsumer struct{}

func (consumer *AddProjectConsumer) Consume(url string) *widget.Project {
	defer func() {
		err := recover()
		if err != nil {
			fmt.Printf("Error adding project: %s\n", err)
		}
	}()

	// perform task
	if env.Get("DEBUG") == "true" {
		log.Printf("Resolving path %s", url)
	}

	var curseId uint

	db, err := GetDatabase()
	if err != nil {
		panic(err)
	}

	project := &widget.Project{}
	err = db.Where("path = ?", url).Find(&project).Error
	if err != nil {
		panic(err)
	}

	//if the path is just an id, that's the curse id
	//otherwise..... we can try a search....?
	if curseId, err = cast.ToUintE(url); err == nil {
		project.CurseId = &curseId
	} else if matches := FullPathWithId.FindStringSubmatch(url); len(matches) > 0 {
		curseId = cast.ToUint(matches[1])
		project.CurseId = &curseId
	} else {
		//for now, we can't resolve, so mark as 404
		id, err := resolveSlug(url)
		if id == 0 {
			project.Status = http.StatusNotFound
			err = db.Save(project).Error
			if err != nil {
				panic(err)
			}
			return project
		} else {
			project.CurseId = &id
		}
	}

	if project.CurseId != nil && *project.CurseId != 0 {
		var count int64
		err = db.Model(&widget.Project{}).Where("curse_id = ? AND status = ?", project.CurseId, http.StatusOK).Count(&count).Error
		if err != nil {
			panic(err)
		}
		if count > 0 {
			project.Status = http.StatusMovedPermanently
		}
	}

	err = db.Save(project).Error
	if err != nil {
		panic(err)
	}

	if project.Status == http.StatusMovedPermanently {
		return project
	}

	return SyncProject(project.ID)
}

func resolveSlug(path string) (uint, error) {
	var err error

	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		return 0, errors.New("invalid slug")
	}

	game := curseforge.GetGameBySlug(parts[0])
	category := parts[1]
	slug := parts[2]

	if game.Id == 0 {
		return 0, errors.New("unknown game")
	}

	categories, err := curseforge.GetCategories(game.Id)
	if err != nil {
		return 0, err
	}

	var classId uint
	for _, v := range categories {
		if v.Slug == category {
			classId = v.Id
		}
	}

	if classId == 0 {
		return 0, errors.New("unknown category")
	}

	response, err := curseforge.Call(fmt.Sprintf("https://api.curseforge.com/v1/mods/search?slug=%s&gameId=%d&classId=%d", slug, game.Id, classId))
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	var data curseforge.SearchResponse
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return 0, err
	}

	for _, v := range data.Data {
		if v.Slug == slug {
			return v.Id, nil
		}
	}

	return 0, errors.New("slug not found")
}
