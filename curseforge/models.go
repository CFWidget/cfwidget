package curseforge

import "time"

type Addon struct {
	Id                uint
	Name              string
	Summary           string
	Authors           []Author
	Categories        []Category
	PrimaryCategoryId uint
	Slug              string
	GameSlug          string
	GameName          string
	DateCreated       time.Time
	DateModified      time.Time
	DateReleased      time.Time
	ModLoaders        []string
	WebsiteUrl        string
	Attachments       []Attachment
	DownloadCount     float64
	CategorySection   CategorySection
}

type Author struct {
	Name              string
	Url               string
	ProjectId         uint
	Id                uint
	ProjectTitleId    uint
	ProjectTitleTitle string
	UserId            uint
	TwitchId          uint
}

type Attachment struct {
	Id           uint
	ProjectId    uint
	Description  string
	IsDefault    bool
	ThumbnailUrl string
	Title        string
	Url          string
	Status       int
}

type File struct {
	Id              uint
	DisplayName     string
	FileName        string
	FileDate        time.Time
	FileLength      uint64
	ReleaseType     int
	FileStatus      int
	DownloadUrl     string
	IsAlternate     bool
	AlternateFileId uint
	GameVersion     []string
	DownloadCount   uint
}

type Category struct {
	CategoryId uint
	Name       string
}

type CategorySection struct {
	Id   uint
	Name string
	Path string
}
