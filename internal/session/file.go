package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mars/marspi-core/llm"
)

var ErrCorrupt = errors.New("session file is corrupted")

type File struct {
	Path string
}

func (file File) Load() ([]llm.Message, error) {
	messages, primaryErr := read(file.Path)
	if primaryErr == nil {
		return messages, nil
	}
	if !errors.Is(primaryErr, os.ErrNotExist) && !errors.Is(primaryErr, ErrCorrupt) {
		return nil, primaryErr
	}
	messages, temporaryErr := read(file.Path + ".tmp")
	if temporaryErr == nil {
		if err := file.Save(messages); err != nil {
			return nil, fmt.Errorf("recover session: %w", err)
		}
		return messages, nil
	}
	if !errors.Is(temporaryErr, os.ErrNotExist) && !errors.Is(temporaryErr, ErrCorrupt) {
		return nil, temporaryErr
	}
	if errors.Is(primaryErr, os.ErrNotExist) && errors.Is(temporaryErr, os.ErrNotExist) {
		return nil, nil
	}
	corrupt := file
	if errors.Is(primaryErr, os.ErrNotExist) {
		corrupt.Path += ".tmp"
	}
	backup, err := corrupt.Backup()
	if err != nil {
		return nil, fmt.Errorf("back up corrupt session: %w", err)
	}
	return nil, fmt.Errorf("%w; backed up to %s", ErrCorrupt, backup)
}

func (file File) Save(messages []llm.Message) error {
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
		return err
	}
	temporary := file.Path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporary, file.Path); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	return nil
}

func (file File) Backup() (string, error) {
	if _, err := os.Stat(file.Path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	backup, err := os.CreateTemp(filepath.Dir(file.Path), filepath.Base(file.Path)+".*.backup")
	if err != nil {
		return "", err
	}
	if err := backup.Close(); err != nil {
		return "", errors.Join(err, os.Remove(backup.Name()))
	}
	if err := os.Rename(file.Path, backup.Name()); err != nil {
		return "", errors.Join(err, os.Remove(backup.Name()))
	}
	return backup.Name(), nil
}

func read(path string) ([]llm.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var messages []llm.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrCorrupt, path, err)
	}
	return messages, nil
}
