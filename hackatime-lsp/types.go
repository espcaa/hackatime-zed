package main

type Heartbeat struct {
	Entity           string  `json:"entity"`
	EntityType       string  `json:"entity_type"`
	Category         string  `json:"category"`
	Time             float64 `json:"time"`
	Plugin           string  `json:"plugin"`
	LineNumber       int     `json:"lineno"`
	CursorPos        int     `json:"cursorpos"`
	Lines            int     `json:"lines_in_file"`
	AlternateProject string  `json:"alternate_project"`
	ProjectFolder    string  `json:"project_folder"`
	IsWrite          bool    `json:"is_write"`
	IsUnsaved        bool    `json:"is_unsaved_entity"`
	LocalFile        string  `json:"local_file,omitempty"`
	Branch           string
	Language         string
	Hostname         string
	UserAgent        string
}
