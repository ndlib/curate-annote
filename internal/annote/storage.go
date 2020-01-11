package annote

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store provides a place to save file content. At some point
// it would be nice for it to provide versioning.
type Store struct {
	Root string
}

var (
	FileStore Store
)

func (s *Store) Create(key string) (io.WriteCloser, error) {
	k := s.resolve(key)
	return os.Create(k)
}

func (s *Store) Open(key string) (io.ReadCloser, error) {
	k := s.resolve(key)
	return os.Open(k)
}

// Find returns an absolute file path to the given item.
func (s *Store) Find(key string) string {
	path, _ := filepath.Abs(s.resolve(key))
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return ""
	}
	return path
}

func (s *Store) resolve(key string) string {
	k := fmt.Sprintf("%s-%04d", key, 0)
	return filepath.Join(s.Root, k)
}

func (s *Store) MakeThumbnailPDF(key string) error {
	t := Thumbnail{Source: s}
	return t.DoPDF(key)
}

func (s *Store) MakeThumbnailImage(key string) error {
	t := Thumbnail{Source: s}
	return t.DoImage(key)
}
