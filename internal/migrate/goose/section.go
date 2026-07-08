package goose

import (
	"errors"
	"strings"
)

func UpSQL(content string) (string, error) {
	lines := strings.Split(content, "\n")
	upLine := -1
	downLine := -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case "-- +goose Up":
			if upLine >= 0 {
				return "", errors.New("found duplicate goose up annotations")
			}
			upLine = i
		case "-- +goose Down":
			if downLine >= 0 {
				return "", errors.New("found duplicate goose down annotations")
			}
			downLine = i
		}
	}
	if upLine < 0 {
		return "", errors.New("missing goose up annotation")
	}
	if downLine >= 0 && downLine < upLine {
		return "", errors.New("goose down annotation must appear after goose up annotation")
	}

	end := len(lines)
	if downLine >= 0 {
		end = downLine
	}
	return strings.TrimSpace(strings.Join(lines[upLine+1:end], "\n")), nil
}
