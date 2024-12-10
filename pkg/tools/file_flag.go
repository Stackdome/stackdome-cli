package tools

import (
	"errors"
	"os"
)

type FileFlag interface {
	Set() error
	UnSet() error
	Raised() (bool, error)
}

type fileFlag struct {
	path string
}

func NewFileFlag(filePath string) FileFlag {
	return &fileFlag{
		path: filePath,
	}
}

func (f *fileFlag) Set() error {
	_, err := os.Create(f.path)
	return err
}

func (f *fileFlag) UnSet() error {
	err := os.Remove(f.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (f *fileFlag) Raised() (bool, error) {
	_, err := os.Stat(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
