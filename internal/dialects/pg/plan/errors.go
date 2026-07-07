package plan

import (
	"errors"
	"fmt"
)

func unsupported(message string) error {
	return fmt.Errorf("%s; semantic planner support is not implemented yet", message)
}

func unsupportedDestructive(message string) error {
	return errors.New(message + "; destructive migration support is not implemented yet")
}
