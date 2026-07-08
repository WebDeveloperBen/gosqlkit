package plan

import "fmt"

func unsupported(message string) error {
	return fmt.Errorf("%s; semantic planner support is not implemented yet", message)
}
