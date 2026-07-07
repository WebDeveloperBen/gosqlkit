package goose

import (
	"errors"
	"fmt"
	"strings"
)

func Validate(content string) error {
	upLine := -1
	downLine := -1
	upCount := 0
	downCount := 0

	for i, line := range strings.Split(content, "\n") {
		switch strings.TrimSpace(line) {
		case "-- +goose Up":
			upCount++
			if upLine == -1 {
				upLine = i
			}
		case "-- +goose Down":
			downCount++
			if downLine == -1 {
				downLine = i
			}
		}
	}

	if upCount == 0 {
		return errors.New("missing goose up annotation")
	}
	if upCount > 1 {
		return fmt.Errorf("found %d goose up annotations, want 1", upCount)
	}
	if downCount > 1 {
		return fmt.Errorf("found %d goose down annotations, want at most 1", downCount)
	}
	if downLine != -1 && downLine < upLine {
		return errors.New("goose down annotation must appear after goose up annotation")
	}
	return nil
}
