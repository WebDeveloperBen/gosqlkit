package plan

import (
	"errors"
	"sort"
)

// MapBy indexes items by a string key. Later items with the same key win.
func MapBy[T any](items []T, key func(T) string) map[string]T {
	out := make(map[string]T, len(items))
	for _, item := range items {
		out[key(item)] = item
	}
	return out
}

// SortedBy returns a copy of items sorted stably by a string key.
func SortedBy[T any](items []T, key func(T) string) []T {
	out := append([]T(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		return key(out[i]) < key(out[j])
	})
	return out
}

// SortChangesByDependencies orders changes so that a create/alter is emitted
// after the objects it depends on, and a drop is emitted before the objects it
// depends on (drops invert the edge direction). It returns an error if the
// dependency graph contains a cycle. This is dialect-neutral: it operates only
// on the shared Change IR.
func SortChangesByDependencies(changes []Change) ([]Change, error) {
	byObject := make(map[ObjectRef]int, len(changes))
	for i, change := range changes {
		if _, ok := byObject[change.Object]; ok {
			continue
		}
		byObject[change.Object] = i
	}

	// For drops the ordering inverts: an object must be dropped before the
	// objects it depends on, so a dropped object's dependents (other drops that
	// reference it) must be emitted first. dropDependents maps an object ref to
	// the drop changes that depend on it.
	dropDependents := make(map[ObjectRef][]int, len(changes))
	for i, change := range changes {
		if change.Op != OperationDrop {
			continue
		}
		for _, dependency := range change.Dependencies {
			dropDependents[dependency] = append(dropDependents[dependency], i)
		}
	}

	visiting := make(map[int]bool, len(changes))
	visited := make(map[int]bool, len(changes))
	sorted := make([]Change, 0, len(changes))

	var visit func(int) error
	visit = func(i int) error {
		if visited[i] {
			return nil
		}
		if visiting[i] {
			return errors.New("migration changes have a dependency cycle")
		}
		visiting[i] = true
		if changes[i].Op == OperationDrop {
			for _, j := range dropDependents[changes[i].Object] {
				if j == i {
					continue
				}
				if err := visit(j); err != nil {
					return err
				}
			}
		} else {
			for _, dependency := range changes[i].Dependencies {
				j, ok := byObject[dependency]
				if !ok || j == i {
					continue
				}
				if changes[j].Op == OperationDrop {
					continue
				}
				if err := visit(j); err != nil {
					return err
				}
			}
		}
		visiting[i] = false
		visited[i] = true
		sorted = append(sorted, changes[i])
		return nil
	}

	for i := range changes {
		if err := visit(i); err != nil {
			return nil, err
		}
	}
	return sorted, nil
}
