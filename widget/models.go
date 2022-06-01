package widget

import (
	"encoding/json"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Project struct {
	ID         uint    `gorm:"primaryKey"`
	CurseId    *uint   `gorm:"index:;index:idx_curseid_status"`
	Path       string  `gorm:"uniqueIndex;type:VARCHAR(191) COLLATE utf8mb3_unicode_ci"`
	Properties *string `gorm:"type:LONGTEXT COLLATE utf8mb4_bin"`
	Status     int     `gorm:"index:;index:idx_status_updatedat;index:idx_curseid_status"`
	CreatedAt  time.Time
	UpdatedAt  time.Time `gorm:"index:;index:idx_status_updatedat"`

	ParsedProjects *ProjectProperties `gorm:"-"`
}

func (p *Project) AfterFind(*gorm.DB) error {
	if p.Properties == nil || *p.Properties == "" {
		return nil
	}

	//for some things, we have a broken JSON due to issues with the data
	//In some scenarios, PHP made arrays for maps when no data, so Go cannot parse this properly
	//As such, we simply ignore errors.
	//These will return no data at the end until it's re-synced
	_ = json.NewDecoder(strings.NewReader(*p.Properties)).Decode(&p.ParsedProjects)

	if p.ParsedProjects.Id == 0 {
		p.ParsedProjects = nil
		return nil
	}

	if p.ParsedProjects.Download != nil && p.ParsedProjects.Download.Id == 0 {
		p.ParsedProjects.Download = nil
	}
	return nil
}

type ProjectProperties struct {
	Id          uint                     `json:"id"`
	Title       string                   `json:"title"`
	Summary     string                   `json:"summary"`
	Description string                   `json:"description"`
	Game        string                   `json:"game"`
	Type        string                   `json:"type"`
	Urls        map[string]string        `json:"urls"`
	Thumbnail   string                   `json:"thumbnail"`
	CreatedAt   time.Time                `json:"created_at"`
	Downloads   map[string]uint64        `json:"downloads"`
	License     string                   `json:"license"`
	Donate      string                   `json:"donate"`
	Categories  []string                 `json:"categories"`
	Members     []ProjectMember          `json:"members"`
	Links       []string                 `json:"links"`
	Files       []ProjectFile            `json:"files"`
	Versions    map[string][]ProjectFile `json:"versions"`
	Download    *ProjectFile             `json:"download,omitempty"`
}

type ProjectMember struct {
	Title    string `json:"title"`
	Username string `json:"username"`
	Id       uint   `json:"id"`
}

type ProjectFile struct {
	Id         uint      `json:"id"`
	Url        string    `json:"url"`
	Display    string    `json:"display"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Version    string    `json:"version"`
	FileSize   uint64    `json:"filesize"`
	Versions   []string  `json:"versions"`
	Downloads  uint      `json:"downloads"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Author struct {
	Id         uint   `gorm:"primaryKey"`
	Username   string `gorm:"index"`
	Properties *string
	MemberId   uint `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time `gorm:"index"`

	ParsedProjects AuthorProperties `gorm:"-"`
}

func (a *Author) AfterFind(*gorm.DB) error {
	if a.Properties == nil || *a.Properties == "" {
		return nil
	}

	//for some things, we have a broken JSON due to issues with the data
	//In some scenarios, PHP made arrays for maps when no data, so Go cannot parse this properly
	//As such, we simply ignore errors.
	//These will return no data at the end until it's re-synced
	_ = json.NewDecoder(strings.NewReader(*a.Properties)).Decode(&a.ParsedProjects)
	if a.ParsedProjects.Projects == nil {
		a.ParsedProjects.Projects = make([]AuthorProject, 0)
	}
	return nil
}

type AuthorProperties struct {
	Projects []AuthorProject `json:"projects"`
}

type AuthorProject struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type AuthorResponse struct {
	Id       uint            `json:"id"`
	Username string          `json:"username"`
	Projects []AuthorProject `json:"projects"`
}
