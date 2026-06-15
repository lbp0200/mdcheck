package main

type Issue struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}
