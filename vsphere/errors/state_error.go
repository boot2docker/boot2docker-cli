package errors

import "fmt"

type InvalidStateError struct {
	vm string
}

func NewInvalidStateError(vm string) error {
	err := InvalidStateError{
		vm: vm,
	}
	return &err
}

func (err *InvalidStateError) Error() string {
	return fmt.Sprintf("Machine %s state invalid", err.vm)
}
