package apifunc

import (
	"time"

	"github.com/suifengpiao14/torm"
)

type ContextApiFunc struct {
	Api     Api
	Torms   torm.Torms
	Project Project
}

func (ContextApiFunc) Deadline() (deadline time.Time, ok bool) {
	return
}

func (ContextApiFunc) Done() <-chan struct{} {
	return nil
}

func (ContextApiFunc) Err() error {
	return nil
}

func (ContextApiFunc) Value(key any) any {
	return nil
}
