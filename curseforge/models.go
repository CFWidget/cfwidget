package curseforge

import "time"

type Addon struct {
	Id                uint
	GameId            uint
	Name              string
	Slug              string
	Links             Links
	Summary           string
	DownloadCount     float64
	PrimaryCategoryId uint
	Categories        []Category
	Authors           []Author
	Logo              Attachment
	Screenshots       []Attachment
	DateCreated       time.Time
	DateModified      time.Time
	DateReleased      time.Time
}

type Links struct {
	WebsiteUrl string
	WikiUrl    string
	IssuesUrl  string
	SourceUrl  string
}

type Author struct {
	Id   uint
	Name string
	Url  string
}

type Attachment struct {
	Id           uint
	Title        string
	Description  string
	ThumbnailUrl string
	Url          string
}

type File struct {
	Id              uint
	IsAvailable     bool
	DisplayName     string
	FileName        string
	ReleaseType     int
	FileStatus      int
	FileDate        time.Time
	FileLength      uint64
	DownloadCount   uint
	DownloadUrl     string
	AlternateFileId uint
	GameVersions    []string
}

type Category struct {
	Id               uint
	Name             string
	ParentCategoryId uint
	Slug             string
	ClassId          uint
}

type Game struct {
	Id   uint
	Name string
	Slug string
}

type Pagination struct {
	Index       int
	PageSize    int
	ResultCount int
	TotalCount  int
}
