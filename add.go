package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cfwidget/cfwidget/curseforge"
	"github.com/cfwidget/cfwidget/env"
	"github.com/spf13/cast"
	"go.elastic.co/apm/v2"
	"log"
	"regexp"
	"strings"
)

var addProjectConsumer AddProjectConsumer
var FullPathWithId = regexp.MustCompile("[a-zA-Z\\-]+/[a-zA-Z\\-]+/([0-9]+)")

type AddProjectConsumer struct{}

func (consumer *AddProjectConsumer) Consume(url string, ctx context.Context) *uint {
	defer func() {
		err := recover()
		if err != nil {
			fmt.Printf("Error adding project: %s\n", err)
		}
	}()

	// perform task
	if env.GetBool("DEBUG") {
		log.Printf("Resolving path %s", url)
	}

	//if the path is just an id, that's the curse id
	//otherwise..... we can try a search....?
	if curseId, err := cast.ToUintE(url); err == nil {
		return &curseId
	} else if matches := FullPathWithId.FindStringSubmatch(url); len(matches) > 0 {
		curseId = cast.ToUint(matches[1])
		return &curseId
	} else {
		id, err := resolveSlug(url, ctx)
		if err != nil {
			panic(err)
		}
		if id != 0 {
			return &id
		}
	}

	return nil
}

func resolveSlug(path string, c context.Context) (uint, error) {
	span, ctx := apm.StartSpan(c, "resolveSlug", "custom")
	defer span.End()

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

	categories, err := curseforge.GetCategories(game.Id, ctx)
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

	response, err := curseforge.Call(fmt.Sprintf("https://api.curseforge.com/v1/mods/search?slug=%s&gameId=%d&classId=%d", slug, game.Id, classId), ctx)
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
