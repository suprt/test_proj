package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Metadata struct {
	Filename  string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FilesystemStore struct {
	rootDir string
}

func NewFilesystemStore(rootDir string) (*FilesystemStore, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, err
	}
	return &FilesystemStore{rootDir: rootDir}, nil
}

// SaveStream writes stream contents to a temporary file and atomically renames it to the final filename.
func (s *FilesystemStore) SaveStream(ctx context.Context, filename string, write func(w io.Writer) (int64, error)) (Metadata, error) {
	if err := validateFilename(filename); err != nil {
		return Metadata{}, err
	}
	if write == nil {
		return Metadata{}, fmt.Errorf("write func is nil")
	}
	tmp, err := os.CreateTemp(s.rootDir, ".upload-*")
	if err != nil {
		return Metadata{}, err
	}
	tmpPath := tmp.Name()

	var written int64
	var werr error
	done := make(chan struct{})
	go func() {
		written, werr = write(tmp)
		if err := tmp.Sync(); err != nil {
			werr = fmt.Errorf("failed to sync temp file: %w", err)
		}
		close(done)
	}()
	select {
	case <-ctx.Done():
		// Дожидаемся завершения горутины перед очисткой
		<-done
		tmp.Close()
		if err := os.Remove(tmpPath); err != nil {
			fmt.Printf("warning: failed to remove temp file %s: %v\n", tmpPath, err)
		}
		return Metadata{}, ctx.Err()
	case <-done:
	}

	// Закрываем файл перед переименованием
	if err := tmp.Close(); err != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			fmt.Printf("warning: failed to remove temp file %s: %v\n", tmpPath, removeErr)
		}
		return Metadata{}, fmt.Errorf("failed to close temp file: %w", err)
	}

	if werr != nil {
		if err := os.Remove(tmpPath); err != nil {
			fmt.Printf("warning: failed to remove temp file %s: %v\n", tmpPath, err)
		}
		return Metadata{}, werr
	}

	dstPath := filepath.Join(s.rootDir, filepath.Base(filename))
	if err := os.Rename(tmpPath, dstPath); err != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			fmt.Printf("warning: failed to remove temp file %s: %v\n", tmpPath, removeErr)
		}
		return Metadata{}, err
	}
	st, err := os.Stat(dstPath)
	if err != nil {
		return Metadata{}, err
	}
	meta := Metadata{
		Filename:  filepath.Base(filename),
		SizeBytes: written,
		CreatedAt: st.ModTime().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.writeMeta(meta); err != nil {
		return Metadata{}, err
	}
	return meta, nil
}

func (s *FilesystemStore) writeMeta(m Metadata) error {
	f, err := os.Create(s.metaPath(m.Filename))
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("warning: failed to close metadata file %s: %v\n", s.metaPath(m.Filename), err)
		}
	}()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&m); err != nil {
		return err
	}
	return nil
}

func (s *FilesystemStore) metaPath(filename string) string {
	return filepath.Join(s.rootDir, fmt.Sprintf("%s.meta.json", filepath.Base(filename)))
}

func (s *FilesystemStore) Open(_ context.Context, filename string) (io.ReadCloser, Metadata, error) {
	if err := validateFilename(filename); err != nil {
		return nil, Metadata{}, err
	}
	path := filepath.Join(s.rootDir, filepath.Base(filename))
	f, err := os.Open(path)
	if err != nil {
		return nil, Metadata{}, err
	}

	var meta Metadata

	// First try to read sidecar file (priority)
	if b, err := os.ReadFile(s.metaPath(filename)); err == nil {
		if json.Unmarshal(b, &meta) == nil {
			// Sidecar read successfully, use it
			return f, meta, nil
		}
	}

	// Fallback to os.Stat() if sidecar is missing/corrupted
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, Metadata{}, fmt.Errorf("failed to stat file: %w", err)
	}

	meta = Metadata{
		Filename:  filepath.Base(filename),
		SizeBytes: st.Size(),
		CreatedAt: st.ModTime().UTC(), // ModTime as fallback
		UpdatedAt: st.ModTime().UTC(),
	}

	return f, meta, nil
}

func (s *FilesystemStore) List(_ context.Context) ([]Metadata, error) {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, err
	}
	var out []Metadata
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".meta.json") {
			continue
		}

		var meta Metadata

		// First try to read sidecar file (priority)
		if b, err := os.ReadFile(s.metaPath(name)); err == nil {
			if json.Unmarshal(b, &meta) == nil {
				// Sidecar read successfully, use it
				out = append(out, meta)
				continue
			}
		}

		// Fallback to os.Stat() if sidecar is missing/corrupted
		st, err := e.Info()
		if err != nil {
			continue
		}

		meta = Metadata{
			Filename:  name,
			SizeBytes: st.Size(),
			CreatedAt: st.ModTime().UTC(), // ModTime as fallback (not true creation time)
			UpdatedAt: st.ModTime().UTC(),
		}

		out = append(out, meta)
	}
	return out, nil
}

func validateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename is required")
	}
	if filepath.Base(name) != name {
		return fmt.Errorf("invalid filename")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid filename")
	}
	if strings.ContainsRune(name, filepath.Separator) {
		return fmt.Errorf("invalid filename")
	}
	return nil
}
