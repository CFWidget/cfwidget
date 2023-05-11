package main

import (
	"context"
	"fmt"
	"github.com/cfwidget/cfwidget/env"
	"github.com/cfwidget/cfwidget/widget"
	"go.elastic.co/apm/v2"
	"gorm.io/gorm"
	"log"
	"time"
)

var syncAuthorConsumer SyncAuthorConsumer
var syncAuthorChan = make(chan uint, 500)

func syncAuthorWorker() {
	for i := range syncAuthorChan {
		process(i)
	}
}

func process(id uint) {
	trans := apm.DefaultTracer().StartTransaction("authorSync", "schedule")
	defer trans.End()

	defer func() {
		err := recover()
		if err != nil {
			trans.Outcome = "failure"
		}
	}()

	ctx := apm.ContextWithTransaction(context.Background(), trans)
	syncAuthorConsumer.Consume(id, ctx)
}

func ScheduleAuthors() {
	db, err := GetDatabase()
	if err != nil {
		log.Printf("Failed to pull authors to sync: %s", err)
		return
	}

	var authors []uint
	err = db.Model(&widget.Author{}).Where("updated_at < ?", time.Now().Add(-1*time.Hour)).Select("member_id").Order("updated_at ASC").Limit(500).Find(&authors).Error
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

func (consumer *SyncAuthorConsumer) Consume(id uint, ctx context.Context) *widget.Author {
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

	db = db.WithContext(ctx)

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
		err = db.Where("id = ?", v.Id).First(project).Error
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
