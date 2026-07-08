package goose

import (
	"errors"
	"strings"
)

func UpSQL(content string) (string, error) {
	return sectionSQL(content, "-- +goose Up", "-- +goose Down")
}

func DownSQL(content string) (string, error) {
	return sectionSQL(content, "-- +goose Down", "")
}

func sectionSQL(content, startMarker, endMarker string) (string, error) {
	lines := strings.Split(content, "\n")
	startLine := -1
	downLine := -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case "-- +goose Up":
			if startMarker == "-- +goose Up" && startLine >= 0 {
				return "", errors.New("found duplicate goose up annotations")
			}
			if startMarker == "-- +goose Up" {
				startLine = i
			}
		case "-- +goose Down":
			if downLine >= 0 {
				return "", errors.New("found duplicate goose down annotations")
			}
			downLine = i
			if startMarker == "-- +goose Down" {
				startLine = i
			}
		}
	}
	if startLine < 0 {
		if startMarker == "-- +goose Down" {
			return "", errors.New("missing goose down annotation")
		}
		return "", errors.New("missing goose up annotation")
	}
	if startMarker == "-- +goose Up" && downLine >= 0 && downLine < startLine {
		return "", errors.New("goose down annotation must appear after goose up annotation")
	}

	end := len(lines)
	if endMarker == "-- +goose Down" && downLine >= 0 {
		end = downLine
	}
	return strings.TrimSpace(strings.Join(lines[startLine+1:end], "\n")), nil
}
