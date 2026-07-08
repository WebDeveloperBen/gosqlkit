package tooling

import "strings"

func SplitSQLStatements(sql string) []string {
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
