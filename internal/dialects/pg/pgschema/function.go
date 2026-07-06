package pgschema

import (
	"sort"
	"strings"
)

type Function struct {
	Cost            *float64           `json:"cost,omitempty"`
	Rows            *int64             `json:"rows,omitempty"`
	Strict          *bool              `json:"strict,omitempty"`
	Configuration   map[string]string  `json:"configuration,omitempty"`
	Schema          string             `json:"schema,omitempty"`
	Name            string             `json:"name"`
	PreviousName    string             `json:"previousName,omitempty"`
	Language        string             `json:"language"`
	ReturnType      string             `json:"returnType"`
	Body            string             `json:"body"`
	Volatility      string             `json:"volatility,omitempty"`
	Parallel        string             `json:"parallel,omitempty"`
	Comment         string             `json:"comment,omitempty"`
	Arguments       []FunctionArgument `json:"arguments,omitempty"`
	SecurityDefiner bool               `json:"securityDefiner,omitempty"`
}

type FunctionArgument struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type"`
	Mode    string `json:"mode,omitempty"`
	Default string `json:"default,omitempty"`
}

func sortedFunctions(input []Function) []Function {
	items := append([]Function(nil), input...)
	for i := range items {
		items[i].Arguments = append([]FunctionArgument(nil), items[i].Arguments...)
		if items[i].Configuration != nil {
			config := make(map[string]string, len(items[i].Configuration))
			for k, v := range items[i].Configuration {
				config[k] = v
			}
			items[i].Configuration = config
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return functionKey(items[i]) < functionKey(items[j])
	})
	return items
}

func functionKey(fn Function) string {
	parts := make([]string, 0, len(fn.Arguments))
	for _, arg := range fn.Arguments {
		if strings.EqualFold(arg.Mode, "OUT") {
			continue
		}
		parts = append(parts, arg.Type)
	}
	return qualified(fn.Schema, fn.Name) + "(" + strings.Join(parts, ", ") + ")"
}
