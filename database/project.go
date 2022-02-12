package database

import (
	"bytes"
	"encoding/json"
	"github.com/jinzhu/gorm"
	"github.com/lordralex/cfwidget"
	"strings"
	"time"
)

type Project struct {
	Id            uint64            `gorm:"id"`
	CurseId       uint              `gorm:"curse_id"`
	Path          string            `gorm:"path"`
	RawProperties string            `gorm:"properties"`
	Properties    *cfwidget.Project `gorm:"-"`
	CreatedAt     time.Time         `gorm:"created_at"`
	UpdatedAt     time.Time         `gorm:"updated_at"`
	LastRequested time.Time         `gorm:"last_requested"`
	Status        int               `gorm:"status"`
}

func (p *Project) BeforeSave() (err error) {
	if p.Properties == nil {
		return
	}

	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(p.Properties)
	if err != nil {
		return
	}

	p.RawProperties = buf.String()
	return
}

func (p *Project) AfterFind() (err error) {
	if p.RawProperties == "" {
		return
	}
	err = json.NewDecoder(strings.NewReader(p.RawProperties)).Decode(p.Properties)
	return
}

func Get(slug string) (project *Project, err error) {
	var db *gorm.DB

	db, err = GetConnection()
	if err != nil {
		return
	}

	project = &Project{Path: slug}
	err = db.Where(project).First(&project).Error
	if err != nil {
		project = nil
	}

	if project != nil && project.Status == 301 && project.CurseId != 0 {
		project = &Project{CurseId: project.CurseId, Status: 200}
		err = db.Where(project).First(&project).Error
		if err != nil {
			project = nil
		}
	}

	return
}

func Save(project *Project) error {
	db, err := GetConnection()
	if err != nil {
		return err
	}

	return db.Save(project).Error
}
