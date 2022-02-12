package cfwidget

import "time"

type Project struct {
	Id          uint              `json:"id"`
	Game        string            `json:"game"`
	Type        string            `json:"type"`
	Urls        map[string]string `json:"urls"`
	Files       []File            `json:"files"`
	Links       []Link            `json:"links"`
	Title       string            `json:"title"`
	Donate      string            `json:"donate,omitempty"`
	License     string            `json:"license"`
	Members     []Member          `json:"members"`
	Versions    map[string][]File `json:"versions"`
	Downloads   Downloads         `json:"downloads"`
	Thumbnail   string            `json:"thumbnail"`
	Categories  []string          `json:"categories"`
	CreatedAt   time.Time         `json:"created_at"`
	Description string            `json:"description"`
}

type File struct {
	Id         uint64    `json:"id"`
	Url        string    `json:"url"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Version    string    `json:"version"`
	FileSize   string    `json:"filesize"`
	Versions   []string  `json:"versions"`
	Downloads  uint64    `json:"downloads"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Link struct {
	Target string `json:"href"`
	Title  string `json:"title"`
}

type Member struct {
	Title    string `json:"title"`
	Username string `json:"username"`
}

type Downloads struct {
	Total   uint64 `json:"total"`
	Monthly uint64 `json:"monthly"`
}
