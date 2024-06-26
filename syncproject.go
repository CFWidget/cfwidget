package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cfwidget/cfwidget/curseforge"
	"github.com/cfwidget/cfwidget/env"
	"github.com/cfwidget/cfwidget/widget"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
)

var syncProjectConsumer SyncProjectConsumer

var remoteUrlRegex = regexp.MustCompile("\"/linkout\\?remoteUrl=(?P<Url>\\S*)\"")

var NoProjectError = errors.New("no such project")
var PrivateProjectError = errors.New("project private")

var invalidVersions = []string{"Forge", "Fabric", "Quilt", "Rift"}

func SyncProject(id uint, ctx context.Context) (*widget.Project, error) {
	//just directly perform the call, we want this one now
	return syncProjectConsumer.Consume(id, ctx)
}

type SyncProjectConsumer struct{}

func (consumer *SyncProjectConsumer) Consume(curseId uint, ctx context.Context) (project *widget.Project, err error) {
	db, err := GetDatabase()
	if err != nil {
		return nil, err
	}
	db = db.WithContext(ctx)

	//let this handle how to mark the job
	//if we get an error, it failed
	//otherwise, it's fine
	defer func() {
		e := recover()
		if e != nil {
			log.Printf("Error syncing project %d: %s\n%s", curseId, e, debug.Stack())
			if project != nil {
				project.Error = cast.ToString(e)
				_ = db.Save(project).Error
			}
			if t, ok := e.(error); ok {
				err = t
			} else {
				err = errors.New(cast.ToString(e))
			}
		}
	}()

	// perform task
	if env.GetBool("DEBUG") {
		log.Printf("Syncing project %d", curseId)
	}

	project = &widget.Project{
		CurseId: curseId,
	}
	err = db.First(project).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		panic(err)
	}

	addon, err := getAddonProperties(curseId, ctx)
	if err != nil {
		if errors.Is(err, NoProjectError) {
			project.Status = 404
		} else if errors.Is(err, PrivateProjectError) {
			project.Status = 403
		} else {
			panic(err)
		}

		err = db.Save(project).Error
		if err != nil {
			panic(err)
		}

		return nil, err
	}

	description, err := getAddonDescription(curseId, ctx)
	if err != nil && !errors.Is(err, NoProjectError) && !errors.Is(err, PrivateProjectError) {
		panic(err)
	}

	newProps := &widget.ProjectProperties{
		Id:          addon.Id,
		Title:       addon.Name,
		Summary:     addon.Summary,
		Description: description,
		Game:        curseforge.GetGame(addon.GameId).Slug,
		Type:        "",
		Urls: map[string]string{
			"curseforge": addon.Links.WebsiteUrl,
			"project":    addon.Links.WebsiteUrl,
		},
		CreatedAt: addon.DateCreated,
		Downloads: map[string]uint64{
			"monthly": 0,
			"total":   cast.ToUint64(addon.DownloadCount),
		},
		License:    "",
		Donate:     "",
		Categories: make([]string, 0),
		Members:    make([]widget.ProjectMember, 0),
		Links:      make([]string, 0),
		Files:      make([]widget.ProjectFile, 0),
		Versions:   map[string][]widget.ProjectFile{},
	}

	for _, v := range addon.Categories {
		newProps.Categories = append(newProps.Categories, v.Name)
	}

	categories, err := curseforge.GetCategories(addon.GameId, ctx)
	newProps.Type = curseforge.GetPrimaryCategoryFor(categories, addon.PrimaryCategoryId).Name

	newProps.Thumbnail = addon.Logo.ThumbnailUrl

	for _, v := range addon.Authors {
		newProps.Members = append(newProps.Members, widget.ProjectMember{
			Username: v.Name,
			Title:    coalesce("Owner"),
			Id:       v.Id,
		})
	}

	//files!!!!
	//we have to call their API to get this stuff
	files, err := curseforge.GetFiles(curseId, ctx)
	if err != nil && !errors.Is(err, NoProjectError) && !errors.Is(err, PrivateProjectError) {
		newProps.Files = project.ParsedProjects.Files
		log.Printf("Error getting files: %s\n%s", err, debug.Stack())
	}

	for _, v := range files {
		//if the file is not a "public" file, skip it
		if !curseforge.IsAllowedFile(v.FileStatus) {
			continue
		}

		file := widget.ProjectFile{
			Id:         v.Id,
			Url:        fmt.Sprintf("%s/files/%d", addon.Links.WebsiteUrl, v.Id),
			Display:    v.DisplayName,
			Name:       v.FileName,
			Type:       curseforge.GetReleaseType(v.ReleaseType),
			Version:    firstOrEmpty(v.GameVersions),
			FileSize:   v.FileLength,
			Versions:   v.GameVersions,
			Downloads:  v.DownloadCount,
			UploadedAt: v.FileDate,
		}

		for _, g := range v.GameVersions {
			if !contains(g, invalidVersions) {
				file.Version = g
				break
			}
		}

		newProps.Files = append(newProps.Files, file)

		for _, ver := range file.Versions {
			if contains(ver, invalidVersions) {
				continue
			}
			d, e := newProps.Versions[ver]
			if !e {
				d = []widget.ProjectFile{file}
			} else {
				d = append(d, file)
			}

			newProps.Versions[ver] = d
		}
	}

	d, err := json.Marshal(newProps)
	if err != nil {
		panic(err)
	}

	s := string(d)
	project.Properties = &s
	project.ParsedProjects = newProps
	project.Status = http.StatusOK

	err = db.Save(project).Error
	if err != nil {
		panic(err)
	}

	//now, update authors to indicate this project is associated with them
	for _, a := range project.ParsedProjects.Members {
		var author widget.Author
		err = db.Where("member_id = ?", a.Id).First(&author).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				author.Username = a.Username
				author.MemberId = a.Id
				temp := "{}"
				author.Properties = &temp
				_ = db.Create(&author).Error
			}
		}

		exists := false
		for _, e := range author.ParsedProjects.Projects {
			if e.Id == project.CurseId {
				exists = true
				break
			}
		}
		if !exists {
			author.ParsedProjects.Projects = append(author.ParsedProjects.Projects, widget.AuthorProject{
				Id:   project.CurseId,
				Name: project.ParsedProjects.Title,
			})

			d, err = json.Marshal(author.ParsedProjects)
			if err != nil {
				continue
			}

			s = string(d)
			author.Properties = &s

			_ = db.Save(&author).Error
		}
	}

	return project, nil
}

func getAddonProperties(id uint, ctx context.Context) (addon curseforge.Addon, err error) {
	u := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d", id)

	response, err := curseforge.Call(u, ctx)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return addon, NoProjectError
	} else if response.StatusCode == http.StatusForbidden {
		return addon, PrivateProjectError
	} else if response.StatusCode != 200 {
		body, _ := io.ReadAll(response.Body)
		return addon, errors.New(fmt.Sprintf("Error from CurseForge properties for id %d: %s (%d)", id, string(body), response.StatusCode))
	}

	var res curseforge.ProjectResponse
	err = json.NewDecoder(response.Body).Decode(&res)
	addon = res.Data
	return
}

func getAddonDescription(id uint, ctx context.Context) (description string, err error) {
	requestUrl := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/description", id)

	response, err := curseforge.Call(requestUrl, ctx)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return "", NoProjectError
	} else if response.StatusCode == http.StatusForbidden {
		return "", PrivateProjectError
	} else if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return description, errors.New(fmt.Sprintf("Error from CurseForge description for id %d: %s (%d)", id, string(body), response.StatusCode))
	}

	var data curseforge.DescriptionResponse
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		return
	}

	description = remoteUrlRegex.ReplaceAllStringFunc(data.Data, func(match string) string {
		urls := remoteUrlRegex.FindStringSubmatch(match)
		if len(urls) < 2 {
			return match
		}
		result := urls[1]
		result, err = url.QueryUnescape(result)
		result, err = url.QueryUnescape(result)
		return "\"" + result + "\""
	})
	return
}
