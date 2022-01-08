package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lordralex/cfwidget/curseforge"
	"github.com/lordralex/cfwidget/widget"
	"github.com/spf13/cast"
	"io"
	"log"
	"net/url"
	"regexp"
)

var syncProjectConsumer SyncProjectConsumer
var syncChan = make(chan uint)

var remoteUrlRegex = regexp.MustCompile("\"/linkout\\?remoteUrl=(?P<Url>\\S*)\"")

func ScheduleProjects() {
	var projects []uint
	err := db.Model(&widget.Project{}).Where("status = ?", 200).Select("id").Order("updated_at ASC").Limit(100).Find(&projects).Error
	if err != nil {
		log.Printf("Failed to pull projects to sync: %s", err)
		return
	}

	for _, v := range projects {
		//kick off a worker to handle this
		syncChan <- v
		if err != nil {
			log.Printf("Failed to queue %d for syncing: %s", v, err)
		}
	}
}

func SyncProject(id uint) {
	//just directly perform the call, we want this one now
	syncProjectConsumer.Consume(id)
}

func syncWorker() {
	for i := range syncChan {
		syncProjectConsumer.Consume(i)
	}
}

type SyncProjectConsumer struct{}

func (consumer *SyncProjectConsumer) Consume(id uint) {
	//let this handle how to mark the job
	//if we get an error, it failed
	//otherwise, it's fine
	defer func() {
		err := recover()
		if err != nil {
			fmt.Printf("Error syncing project: %s\n", err)
		}
	}()

	// perform task
	log.Printf("Syncing project %d", id)

	project := &widget.Project{}
	err := db.First(&project, id).Error
	if err != nil {
		panic(err)
	}

	var curseId uint
	curseId = *project.CurseId

	addon, err := getAddonProperties(curseId)
	if err != nil {
		panic(err)
	}

	description, err := getAddonDescription(curseId)
	if err != nil {
		panic(err)
	}

	newProps := widget.ProjectProperties{
		Id:          addon.Id,
		Title:       addon.Name,
		Summary:     addon.Summary,
		Description: description,
		Game:        addon.GameSlug,
		Type:        addon.CategorySection.Name,
		Urls: map[string]string{
			"curseforge": addon.WebsiteUrl,
			"project":    addon.WebsiteUrl,
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

	var thumbnail string
	for _, v := range addon.Attachments {
		if v.IsDefault {
			thumbnail = v.ThumbnailUrl
		}
	}
	newProps.Thumbnail = thumbnail

	for _, v := range addon.Authors {
		newProps.Members = append(newProps.Members, widget.ProjectMember{
			Username: v.Name,
			Title:    coalesce(v.ProjectTitleTitle, "Owner"),
			Id:       v.UserId,
		})
	}

	//files!!!!
	//we have to call their API to get this stuff
	files, err := getAddonFiles(curseId)
	if err != nil {
		panic(err)
	}

	for _, v := range files {
		//if the file is not a "public" file, skip it
		if !curseforge.IsAllowedFile(v.FileStatus) {
			continue
		}

		file := widget.ProjectFile{
			Id:         v.Id,
			Url:        fmt.Sprintf("%s/files/%d", addon.WebsiteUrl, v.Id),
			Display:    v.DisplayName,
			Name:       v.FileName,
			Type:       curseforge.GetReleaseType(v.ReleaseType),
			Version:    firstOrEmpty(v.GameVersion),
			FileSize:   v.FileLength,
			Versions:   v.GameVersion,
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

	err = db.Save(project).Error
	if err != nil {
		panic(err)
	}
}

func getAddonProperties(id uint) (addon curseforge.Addon, err error) {
	url := fmt.Sprintf("https://addons-ecs.forgesvc.net/api/v2/addon/%d", id)

	response, err := client.Get(url)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		//we have a bad status from curseforge.... we probably should just skip and move on
		var data []byte
		data, err = io.ReadAll(response.Body)
		if err != nil {
			return
		}

		return addon, errors.New(fmt.Sprintf("Error from CurseForge for id %d: %s (%d)", id, string(data), response.StatusCode))
	}

	err = json.NewDecoder(response.Body).Decode(&addon)
	return
}

func getAddonFiles(id uint) (files []curseforge.File, err error) {
	url := fmt.Sprintf("https://addons-ecs.forgesvc.net/api/v2/addon/%d/files", id)

	response, err := client.Get(url)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		//we have a bad status from curseforge.... we probably should just skip and move on
		var data []byte
		data, err = io.ReadAll(response.Body)
		if err != nil {
			return
		}

		return files, errors.New(fmt.Sprintf("Error from CurseForge for id %d: %s (%d)", id, string(data), response.StatusCode))
	}

	err = json.NewDecoder(response.Body).Decode(&files)
	return
}

func getAddonDescription(id uint) (description string, err error) {
	requestUrl := fmt.Sprintf("https://addons-ecs.forgesvc.net/api/v2/addon/%d/description", id)

	response, err := client.Get(requestUrl)
	if err != nil {
		return
	}
	defer response.Body.Close()

	var data []byte
	data, err = io.ReadAll(response.Body)

	if response.StatusCode != 200 {
		return description, errors.New(fmt.Sprintf("Error from CurseForge for id %d: %s (%d)", id, string(data), response.StatusCode))
	}

	description = remoteUrlRegex.ReplaceAllStringFunc(string(data), func(match string) string {
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
