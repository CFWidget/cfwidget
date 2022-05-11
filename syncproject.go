package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lordralex/cfwidget/curseforge"
	"github.com/lordralex/cfwidget/env"
	"github.com/lordralex/cfwidget/widget"
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
var syncChan = make(chan uint, 500)

var remoteUrlRegex = regexp.MustCompile("\"/linkout\\?remoteUrl=(?P<Url>\\S*)\"")
var authorIdRegex = regexp.MustCompile("https://www\\.curseforge\\.com/members/(?P<ID>[0-9]+)-")

var NoProjectError = errors.New("no such project")
var PrivateProjectError = errors.New("project private")

func SyncProject(id uint) *widget.Project {
	//just directly perform the call, we want this one now
	return syncProjectConsumer.Consume(id)
}

func syncWorker() {
	for i := range syncChan {
		_ = syncProjectConsumer.Consume(i)
	}
}

type SyncProjectConsumer struct{}

func (consumer *SyncProjectConsumer) Consume(id uint) *widget.Project {
	//let this handle how to mark the job
	//if we get an error, it failed
	//otherwise, it's fine
	defer func() {
		err := recover()
		if err != nil {
			log.Printf("Error syncing project %d: %s\n%s", id, err, debug.Stack())
		}
	}()

	db, err := GetDatabase()
	if err != nil {
		panic(err)
	}

	// perform task
	if env.Get("DEBUG") == "true" {
		log.Printf("Syncing project %d", id)
	}

	project := &widget.Project{}
	err = db.First(project, id).Error
	if err != nil {
		panic(err)
	}

	if project.CurseId == nil {
		project.Status = 404
		err = db.Save(project).Error
		if err != nil {
			panic(err)
		}
		return project
	}

	var curseId *uint
	curseId = project.CurseId

	addon, err := getAddonProperties(*curseId)
	if err != nil {
		if err == NoProjectError {
			project.Status = 404
			err = db.Save(project).Error
			if err != nil {
				panic(err)
			}
		} else if err == PrivateProjectError {
			project.Status = 403
			err = db.Save(project).Error
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}

		return project
	}

	description, err := getAddonDescription(*curseId)
	if err != nil && err != NoProjectError && err != PrivateProjectError {
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

	categories, err := curseforge.GetCategories(addon.GameId)
	newProps.Type = curseforge.GetPrimaryCategoryFor(categories, addon.PrimaryCategoryId).Name

	newProps.Thumbnail = addon.Logo.ThumbnailUrl

	for _, v := range addon.Authors {
		var authorId uint

		urls := authorIdRegex.FindStringSubmatch(v.Url)
		if len(urls) < 2 {
			authorId = v.Id
		}
		authorId = cast.ToUint(urls[1])
		newProps.Members = append(newProps.Members, widget.ProjectMember{
			Username: v.Name,
			Title:    coalesce("Owner"),
			Id:       authorId,
		})
	}

	//files!!!!
	//we have to call their API to get this stuff
	files, err := curseforge.GetFiles(*curseId)
	if err != nil && err != NoProjectError && err != PrivateProjectError {
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

		newProps.Files = append(newProps.Files, file)

		for _, ver := range file.Versions {
			if ver == "Forge" {
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
		if err != nil && err != gorm.ErrRecordNotFound {
			panic(err)
		} else if err == gorm.ErrRecordNotFound {
			author.Username = a.Username
			author.MemberId = a.Id
			temp := "{}"
			author.Properties = &temp
			err = db.Create(&author).Error
			if err != nil {
				panic(err)
			}
		}
		exists := false
		for _, e := range author.ParsedProjects.Projects {
			if e.Id == *project.CurseId {
				exists = true
				break
			}
		}
		if !exists {
			author.ParsedProjects.Projects = append(author.ParsedProjects.Projects, widget.AuthorProject{
				Id:   *project.CurseId,
				Name: project.ParsedProjects.Title,
			})

			d, err = json.Marshal(author.ParsedProjects)
			if err != nil {
				panic(err)
			}

			s = string(d)
			author.Properties = &s

			err = db.Save(&author).Error
			if err != nil {
				panic(err)
			}
		}
	}

	return project
}

func getAddonProperties(id uint) (addon curseforge.Addon, err error) {
	u := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d", id)

	response, err := curseforge.Call(u)
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

func getAddonDescription(id uint) (description string, err error) {
	requestUrl := fmt.Sprintf("https://api.curseforge.com/v1/mods/%d/description", id)

	response, err := curseforge.Call(requestUrl)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return "", NoProjectError
	} else if response.StatusCode == http.StatusForbidden {
		return "", PrivateProjectError
	} else if response.StatusCode != 200 {
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
