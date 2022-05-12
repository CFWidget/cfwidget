package main

import (
	"fmt"
	"github.com/lordralex/cfwidget/env"
	"github.com/lordralex/cfwidget/widget"
	"gorm.io/gorm"
	"log"
	"time"
)

var syncAuthorConsumer SyncAuthorConsumer
var syncAuthorChan = make(chan uint, 500)

func syncAuthorWorker() {
	for i := range syncAuthorChan {
		syncAuthorConsumer.Consume(i)
	}
}

func ScheduleAuthors() {
	db, err := GetDatabase()
	if err != nil {
		log.Printf("Failed to pull authors to sync: %s", err)
		return
	}

	var authors []uint
	err = db.Model(&widget.Author{}).Where("updated_at < ?", time.Now().Add(-1*time.Hour)).Select("id").Order("updated_at ASC").Limit(500).Find(&authors).Error
	if err != nil {
		log.Printf("Failed to pull authors to sync: %s", err)
		return
	}

	for _, v := range authors {
		//kick off a worker to handle this
		syncAuthorChan <- v
	}
}

type SyncAuthorConsumer struct{}

func (consumer *SyncAuthorConsumer) Consume(id uint) *widget.Author {
	//let this handle how to mark the job
	//if we get an error, it failed
	//otherwise, it's fine
	defer func() {
		err := recover()
		if err != nil {
			fmt.Printf("Error syncing author: %s\n", err)
		}
	}()

	db, err := GetDatabase()
	if err != nil {
		panic(err)
	}

	// perform task
	if env.GetBool("DEBUG") {
		log.Printf("Syncing author %d", id)
	}

	author := &widget.Author{}
	err = db.First(&author, id).Error
	if err != nil {
		panic(err)
	}

	newMap := make([]widget.AuthorProject, 0)

	for _, v := range author.ParsedProjects.Projects {
		project := &widget.Project{}
		//we check for a 403 because the project is "abandoned" and this breaks on the new API
		//we will assume the list we have is still okay for them
		err = db.Where("curse_id = ? AND status IN (200, 403)", v.Id).First(project).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			panic(err)
		}

		if err == gorm.ErrRecordNotFound {
			continue
		}

		for _, m := range project.ParsedProjects.Members {
			if m.Id == author.MemberId {
				newMap = append(newMap, v)
				break
			}
		}
	}

	author.ParsedProjects.Projects = newMap
	err = db.Save(&author).Error
	if err != nil {
		panic(err)
	}

	return author
}
