package plan

import (
	"errors"
	"fmt"
	"reflect"
)

func rejectChangedCollection[T any](name string, previous, current []T) error {
	if reflect.DeepEqual(previous, current) {
		return nil
	}
	return unsupported(name + " changed")
}

func unsupported(message string) error {
	return fmt.Errorf("%s; semantic planner support is not implemented yet", message)
}

func unsupportedDestructive(message string) error {
	return errors.New(message + "; destructive migration support is not implemented yet")
}
