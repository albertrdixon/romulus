package romulus

import "fmt"

// Error is a simple type to wrap an error
type Error struct {
	m  string
	oe error
}

func (e Error) Error() string {
	if e.oe == nil {
		return e.m
	}
	return fmt.Sprintf("%s: %s", e.m, e.oe.Error())
}
