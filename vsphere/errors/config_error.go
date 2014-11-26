package errors

import "fmt"

type IncompleteVcConfigError struct {
	component string
}

func NewIncompleteVcConfigError(component string) error {
	err := IncompleteVcConfigError{
		component: component,
	}
	return &err
}

func (err *IncompleteVcConfigError) Error() string {
	return fmt.Sprintf("Incomplete vCenter information: missing %s", err.component)
}
