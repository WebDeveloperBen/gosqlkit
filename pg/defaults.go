package pg

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (c *Column) DefaultString(value string) *Column {
	c.def.Default = quoteLiteral(value)
	return c
}

func (c *Column) DefaultInt(value int) *Column {
	c.def.Default = strconv.Itoa(value)
	return c
}

func (c *Column) DefaultInt64(value int64) *Column {
	c.def.Default = strconv.FormatInt(value, 10)
	return c
}

func (c *Column) DefaultBool(value bool) *Column {
	c.def.Default = strconv.FormatBool(value)
	return c
}

func (c *Column) DefaultJSON(value any) *Column {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("default json value: %v", err))
	}
	c.def.Default = quoteLiteral(string(data)) + "::jsonb"
	return c
}

func (c *Column) DefaultTextArray(values ...string) *Column {
	c.def.Default = quoteLiteral("{" + strings.Join(escapeArrayElements(values), ",") + "}")
	return c
}

func (c *Column) DefaultDate(value time.Time) *Column {
	c.def.Default = quoteLiteral(value.Format("2006-01-02"))
	return c
}

func (c *Column) DefaultTimestamp(value time.Time) *Column {
	c.def.Default = quoteLiteral(value.Format("2006-01-02 15:04:05.999999"))
	return c
}

func (c *Column) DefaultTimestampTZ(value time.Time) *Column {
	c.def.Default = quoteLiteral(value.Format("2006-01-02T15:04:05.999999Z07:00"))
	return c
}

func escapeArrayElements(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.ContainsAny(value, `,"\`) {
			out = append(out, strings.ReplaceAll(value, `\`, `\\`))
			out[len(out)-1] = `"` + strings.ReplaceAll(out[len(out)-1], `"`, `\"`) + `"`
		} else {
			out = append(out, value)
		}
	}
	return out
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
