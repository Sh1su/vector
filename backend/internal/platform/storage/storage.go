// Package storage speichert Binärobjekte unter generierten Schlüsseln (ADR-017).
// Nutzer-Dateinamen tauchen nie im Pfad auf.
package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var reKey = regexp.MustCompile(`^[0-9a-f-]{36}(\.[a-z]+(\.[a-z]+)?)?$`)

// ErrKey meldet einen ungültigen Schlüssel.
var ErrKey = errors.New("invalid storage key")

// Local legt Objekte im Dateisystem ab (Default-Backend).
type Local struct{ Dir string }

func (l Local) path(key string) (string, error) {
	if !reKey.MatchString(key) {
		return "", ErrKey
	}
	return filepath.Join(l.Dir, key[:2], key), nil
}

// Put schreibt ein Objekt atomar (temporäre Datei, dann Umbenennen).
func (l Local) Put(key string, r io.Reader) (int64, error) {
	p, err := l.path(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return n, err
	}
	return n, os.Rename(tmp.Name(), p)
}

// Open öffnet ein Objekt zum Lesen.
func (l Local) Open(key string) (*os.File, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

// Delete entfernt ein Objekt (für abgebrochene Uploads; Aufräumen nach Frist per Job).
func (l Local) Delete(key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	return os.Remove(p)
}
