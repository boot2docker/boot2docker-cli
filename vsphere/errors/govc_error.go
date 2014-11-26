package errors

import "fmt"

type GovcNotFoundError struct {
	path string
}

func NewGovcNotFoundError(path string) error {
	err := GovcNotFoundError{
		path: path,
	}
	return &err
}

func (err *GovcNotFoundError) Error() string {
	return fmt.Sprintf("govc not found: %s", err.path)
}
