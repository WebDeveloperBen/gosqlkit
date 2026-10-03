package sqlite

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

// DefaultBool stores the SQLite integer encoding of a boolean (0 or 1).
func (c *Column) DefaultBool(value bool) *Column {
	if value {
		c.def.Default = "1"
	} else {
		c.def.Default = "0"
	}
	return c
}

func (c *Column) DefaultFloat32(value float32) *Column {
	c.def.Default = strconv.FormatFloat(float64(value), 'f', -1, 32)
	return c
}

func (c *Column) DefaultFloat64(value float64) *Column {
	c.def.Default = strconv.FormatFloat(value, 'f', -1, 64)
	return c
}

// DefaultJSON stores a JSON document as a quoted TEXT literal. SQLite's JSON1
// functions operate over TEXT.
func (c *Column) DefaultJSON(value any) *Column {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("default json value: %v", err))
	}
	c.def.Default = quoteLiteral(string(data))
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

// DefaultCurrentTimestamp uses SQLite's CURRENT_TIMESTAMP keyword (rendered
// verbatim, not quoted).
func (c *Column) DefaultCurrentTimestamp() *Column {
	c.def.Default = "CURRENT_TIMESTAMP"
	return c
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
