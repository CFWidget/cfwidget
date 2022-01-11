package main

import (
	"fmt"
	"github.com/lordralex/cfwidget/widget"
	"github.com/spf13/cast"
	"log"
	"net/http"
	"regexp"
)

var addChan = make(chan string, 10)
var addProjectConsumer AddProjectConsumer
var FullPathWithId = regexp.MustCompile("[a-zA-Z\\-]+/[a-zA-Z\\-]+/([0-9]+)")

type AddProjectConsumer struct{}

func (consumer *AddProjectConsumer) Consume(url string) {
	defer func() {
		err := recover()
		if err != nil {
			fmt.Printf("Error adding project: %s\n", err)
		}
	}()

	// perform task
	log.Printf("Resolving path %s", url)

	var curseId uint
	var err error

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
		//for now, we can't resolve, so mark as 4o4
		project.Status = http.StatusNotFound
		err = db.Save(project).Error
		if err != nil {
			panic(err)
		}
		return
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
		return
	}

	SyncProject(project.ID)
}

func addWorker() {
	for i := range addChan {
		addProjectConsumer.Consume(i)
	}
}
