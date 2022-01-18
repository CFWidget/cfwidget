package curseforge

type ProjectResponse struct {
	Data Addon
}

type DescriptionResponse struct {
	Data string
}

type FilesResponse struct {
	Data []File
}

type GameResponse struct {
	Data []Game
}

type CategoryResponse struct {
	Data []Category
}
