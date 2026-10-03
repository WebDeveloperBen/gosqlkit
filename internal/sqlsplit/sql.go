package sqlsplit

import (
	"regexp"
	"strings"
)

var createTriggerPattern = regexp.MustCompile(`(?is)^\s*(?:--[^\r\n]*(?:\r?\n|$)\s*|/\*.*?\*/\s*)*CREATE\s+(?:TEMP(?:ORARY)?\s+)?TRIGGER\b`)

func Statements(sql string) []string {
	var out []string
	var b strings.Builder
	inString := false
	inQuotedIdentifier := false
	inLineComment := false
	inBlockComment := false
	dollarTag := ""
	for i := 0; i < len(sql); i++ {
		if inLineComment {
			b.WriteByte(sql[i])
			if sql[i] == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if i+1 < len(sql) && sql[i:i+2] == "*/" {
				b.WriteString("*/")
				i++
				inBlockComment = false
				continue
			}
			b.WriteByte(sql[i])
			continue
		}
		if dollarTag != "" {
			if strings.HasPrefix(sql[i:], dollarTag) {
				b.WriteString(dollarTag)
				i += len(dollarTag) - 1
				dollarTag = ""
				continue
			}
			b.WriteByte(sql[i])
			continue
		}
		if inString {
			b.WriteByte(sql[i])
			if sql[i] == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					b.WriteByte(sql[i+1])
					i++
					continue
				}
				inString = false
			}
			continue
		}
		if inQuotedIdentifier {
			b.WriteByte(sql[i])
			if sql[i] == '"' {
				if i+1 < len(sql) && sql[i+1] == '"' {
					b.WriteByte(sql[i+1])
					i++
					continue
				}
				inQuotedIdentifier = false
			}
			continue
		}
		if i+1 < len(sql) && sql[i:i+2] == "--" {
			inLineComment = true
			b.WriteString("--")
			i++
			continue
		}
		if i+1 < len(sql) && sql[i:i+2] == "/*" {
			inBlockComment = true
			b.WriteString("/*")
			i++
			continue
		}
		if tag, ok := dollarQuoteTag(sql[i:]); ok {
			dollarTag = tag
			b.WriteString(tag)
			i += len(tag) - 1
			continue
		}
		switch sql[i] {
		case '\'':
			inString = true
		case '"':
			inQuotedIdentifier = true
		case ';':
			statement := strings.TrimSpace(b.String())
			if statement != "" {
				out = append(out, statement+";")
			}
			b.Reset()
			continue
		}
		b.WriteByte(sql[i])
	}
	if statement := strings.TrimSpace(b.String()); statement != "" {
		out = append(out, statement)
	}
	return out
}

// StatementsWithTriggers splits SQL while keeping trigger bodies atomic.
func StatementsWithTriggers(sql string) []string {
	parts := Statements(sql)
	statements := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if !IsTriggerStatement(part) {
			statements = append(statements, part)
			continue
		}
		var trigger strings.Builder
		trigger.WriteString(part)
		for !triggerStatementEnd(part) {
			i++
			if i == len(parts) {
				break
			}
			part = parts[i]
			trigger.WriteByte('\n')
			trigger.WriteString(part)
		}
		statements = append(statements, trigger.String())
	}
	return statements
}

// IsTriggerStatement reports whether a SQL fragment starts with CREATE TRIGGER.
func IsTriggerStatement(statement string) bool {
	return createTriggerPattern.MatchString(statement)
}

func triggerStatementEnd(statement string) bool {
	statement = strings.TrimSpace(statement)
	if len(statement) < len("END") || !strings.EqualFold(statement[:len("END")], "END") {
		return false
	}
	rest := strings.TrimSpace(statement[len("END"):])
	return rest == "" || strings.HasPrefix(rest, ";") || strings.HasPrefix(rest, "--") || strings.HasPrefix(rest, "/*")
}

func dollarQuoteTag(value string) (string, bool) {
	if value == "" || value[0] != '$' {
		return "", false
	}
	for i := 1; i < len(value); i++ {
		switch {
		case value[i] == '$':
			return value[:i+1], true
		case value[i] >= 'a' && value[i] <= 'z':
		case value[i] >= 'A' && value[i] <= 'Z':
		case value[i] >= '0' && value[i] <= '9':
		case value[i] == '_':
		default:
			return "", false
		}
	}
	return "", false
}
