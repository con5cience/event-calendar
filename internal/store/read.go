// Package store provides the local artifact-store boundary shared by readers and writers.
package store

import (
	"crypto/sha256"
	"event-calendar/internal/artifact"
	"fmt"
	"io"
	"os"
)

const MaxGenerationBytes = 64 << 20

func ReadDocument(root *os.Root, name string) ([]byte, error) {
	info, err := root.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	b, err := io.ReadAll(io.LimitReader(f, artifact.MaxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > artifact.MaxDocumentBytes {
		return nil, fmt.Errorf("%s exceeds 4 MiB", name)
	}
	return b, nil
}

// Validate checks a complete catalog generation using the supplied byte reader.
func Validate(b []byte, read func(string) ([]byte, error)) (artifact.Catalog, []artifact.Artifact, error) {
	c, err := artifact.DecodeCatalog(b)
	if err != nil {
		return c, nil, fmt.Errorf("catalog: %w", err)
	}
	next := make([]artifact.Artifact, 0, len(c.Sources))
	paths := map[string]bool{}
	total := len(b)
	for _, ref := range c.Sources {
		b, err := read(ref.Artifact)
		if err != nil {
			return c, nil, err
		}
		total += len(b)
		if total > MaxGenerationBytes {
			return c, nil, fmt.Errorf("catalog generation exceeds 64 MiB")
		}
		if fmt.Sprintf("%x", sha256.Sum256(b)) != ref.SHA256 {
			return c, nil, fmt.Errorf("source %s checksum mismatch", ref.SourceID)
		}
		a, err := artifact.DecodeArtifact(b)
		if err != nil {
			return c, nil, fmt.Errorf("source %s: %w", ref.SourceID, err)
		}
		if a.Source.ID != ref.SourceID {
			return c, nil, fmt.Errorf("source identity differs from catalog")
		}
		for _, e := range a.Events {
			if paths[e.PublicPath] {
				return c, nil, fmt.Errorf("duplicate public path across sources")
			}
			paths[e.PublicPath] = true
		}
		next = append(next, a)
	}
	return c, next, nil
}
func Load(root *os.Root) (artifact.Catalog, []artifact.Artifact, error) {
	b, err := ReadDocument(root, "catalog.json")
	if err != nil {
		return artifact.Catalog{}, nil, err
	}
	return Validate(b, func(name string) ([]byte, error) { return ReadDocument(root, name) })
}
