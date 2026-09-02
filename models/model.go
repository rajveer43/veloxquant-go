// Package models provides a curated registry of known local LLMs and
// task-based recommendations.
package models

import "github.com/rajveer43/veloxquant-go/memory"

// Task identifies a category of workload used for model recommendations.
type Task string

const (
	TaskCoding      Task = "coding"
	TaskChat        Task = "chat"
	TaskReasoning   Task = "reasoning"
	TaskVision      Task = "vision"
	TaskAgent       Task = "agent"
	TaskTranslation Task = "translation"
)

// Info describes a model available for local inference.
type Info struct {
	Name string

	Parameters int64

	Architecture memory.Architecture

	Tasks []Task

	Supported   bool
	Recommended bool
}
