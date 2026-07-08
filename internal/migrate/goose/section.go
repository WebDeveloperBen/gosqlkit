package goose

import (
	"errors"
	"fmt"
	"strings"

	"github.com/webdeveloperben/gosqlkit/internal/sqlsplit"
)

func UpSQL(content string) (string, error) {
	statements, err := UpStatements(content)
	if err != nil {
		return "", err
	}
	return strings.Join(statements, "\n"), nil
}

func DownSQL(content string) (string, error) {
	statements, err := DownStatements(content)
	if err != nil {
		return "", err
	}
	return strings.Join(statements, "\n"), nil
}

func UpStatements(content string) ([]string, error) {
	return sectionStatements(content, "-- +goose Up", "-- +goose Down")
}

func DownStatements(content string) ([]string, error) {
	return sectionStatements(content, "-- +goose Down", "")
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

func sectionStatements(content, startMarker, endMarker string) ([]string, error) {
	raw, err := sectionSQL(content, startMarker, endMarker)
	if err != nil {
		return nil, err
	}

	var statements []string
	var normal strings.Builder
	var block strings.Builder
	inBlock := false

	flushNormal := func() {
		statements = append(statements, sqlsplit.Statements(normal.String())...)
		normal.Reset()
	}

	for lineNumber, line := range strings.Split(raw, "\n") {
		switch strings.TrimSpace(line) {
		case "-- +goose StatementBegin":
			if inBlock {
				return nil, fmt.Errorf("nested goose statement begin at line %d", lineNumber+1)
			}
			flushNormal()
			inBlock = true
			continue
		case "-- +goose StatementEnd":
			if !inBlock {
				return nil, fmt.Errorf("goose statement end without statement begin at line %d", lineNumber+1)
			}
			if statement := strings.TrimSpace(block.String()); statement != "" {
				statements = append(statements, statement)
			}
			block.Reset()
			inBlock = false
			continue
		}

		if inBlock {
			block.WriteString(line)
			block.WriteByte('\n')
			continue
		}
		normal.WriteString(line)
		normal.WriteByte('\n')
	}

	if inBlock {
		return nil, errors.New("goose statement begin without statement end")
	}
	flushNormal()
	return statements, nil
}
