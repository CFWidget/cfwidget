package curseforge

type Response struct{}

type PagedResponse struct {
	Response
	Pagination Pagination
}

type ProjectResponse struct {
	Response
	Data Addon
}

type SearchResponse struct {
	PagedResponse
	Data []Addon
}

type DescriptionResponse struct {
	Response
	Data string
}

type FilesResponse struct {
	PagedResponse
	Data []File
}

type GameResponse struct {
	PagedResponse
	Data []Game
}

type CategoryResponse struct {
	PagedResponse
	Data []Category
}
