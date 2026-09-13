package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"event-calendar/internal/artifact"
	"fmt"
	"os"
	"sort"
	"time"
)

type Report struct {
	Generation string `json:"generation,omitempty"`
	Published  bool   `json:"published"`
	Durable    bool   `json:"durable"`
	Error      string `json:"error,omitempty"`
}

// Publisher writes only local files. Inputs must already be reconciled artifacts.
// Private hooks allow deterministic fault injection without production CLI switches.
type Publisher struct {
	generation func() (string, error)
	checkpoint func(string) error
}

func newGeneration() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "g-" + hex.EncodeToString(b[:]), nil
}
func (p *Publisher) check(stage string) error {
	if p.checkpoint != nil {
		return p.checkpoint(stage)
	}
	return nil
}
func syncDir(root *os.Root, name string) error {
	f, err := root.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func writeExclusive(root *os.Root, name string, b []byte) error {
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (p *Publisher) Publish(dir, expected string, inputs [][]byte) (report Report, err error) {
	if expected == "" || len(inputs) == 0 {
		return report, fmt.Errorf("expected generation and at least one candidate are required")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return report, err
	}
	defer root.Close()
	lock, err := lockWriter(root)
	if err != nil {
		return report, err
	}
	defer lock.Close()
	prior := artifact.Catalog{SchemaVersion: 1, Sources: []artifact.ArtifactRef{}}
	priorArtifacts := []artifact.Artifact{}
	old, readErr := ReadDocument(root, "catalog.json")
	if readErr != nil {
		_, statErr := root.Lstat("catalog.json")
		if expected != "none" || !os.IsNotExist(readErr) || !os.IsNotExist(statErr) {
			return report, fmt.Errorf("prior catalog: %w", readErr)
		}
	} else {
		prior, priorArtifacts, err = Validate(old, func(name string) ([]byte, error) { return ReadDocument(root, name) })
		if err != nil {
			return report, fmt.Errorf("prior generation: %w", err)
		}
		if expected == "none" || expected != prior.Generation {
			return report, fmt.Errorf("catalog generation changed; expected %s, found %s", expected, prior.Generation)
		}
	}
	gen := p.generation
	if gen == nil {
		gen = newGeneration
	}
	report.Generation, err = gen()
	if err != nil {
		return report, err
	}
	if report.Generation == prior.Generation {
		return report, fmt.Errorf("generation must be new")
	}
	next := artifact.Catalog{SchemaVersion: 1, Generation: report.Generation, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Sources: []artifact.ArtifactRef{}}
	refs := map[string]artifact.ArtifactRef{}
	known := map[string]artifact.Artifact{}
	for _, ref := range prior.Sources {
		refs[ref.SourceID] = ref
	}
	for _, a := range priorArtifacts {
		known[a.Source.ID] = a
	}
	writes := map[string][]byte{}
	seen := map[string]bool{}
	total := 0
	for _, input := range inputs {
		total += len(input)
		if total > MaxGenerationBytes {
			return report, fmt.Errorf("candidates exceed 64 MiB")
		}
		a, err := artifact.DecodeArtifact(input)
		if err != nil {
			return report, fmt.Errorf("candidate: %w", err)
		}
		if seen[a.Source.ID] {
			return report, fmt.Errorf("duplicate candidate source")
		}
		seen[a.Source.ID] = true
		if old, ok := known[a.Source.ID]; ok && (old.Source != a.Source || old.Venue.Key != a.Venue.Key) {
			return report, fmt.Errorf("candidate changed source or default venue identity")
		}
		// Legacy candidates remain readable, but newly written artifacts omit prices.
		for _, e := range a.Events {
			if e.Price != nil {
				input, err = artifact.EncodeArtifact(artifact.WithoutPrices(a))
				if err != nil {
					return report, err
				}
				break
			}
		}
		name := "sources/" + a.Source.ID + "/" + report.Generation + ".json"
		refs[a.Source.ID] = artifact.ArtifactRef{SourceID: a.Source.ID, Artifact: name, SHA256: fmt.Sprintf("%x", sha256.Sum256(input))}
		writes[name] = input
	}
	for _, ref := range refs {
		next.Sources = append(next.Sources, ref)
	}
	sort.Slice(next.Sources, func(i, j int) bool { return next.Sources[i].SourceID < next.Sources[j].SourceID })
	catalog, err := json.Marshal(next)
	if err != nil {
		return report, err
	}
	catalog = append(catalog, '\n')
	if _, _, err = Validate(catalog, func(name string) ([]byte, error) {
		if b, ok := writes[name]; ok {
			return b, nil
		}
		return ReadDocument(root, name)
	}); err != nil {
		return report, err
	}
	for _, ref := range next.Sources {
		b, ok := writes[ref.Artifact]
		if !ok {
			continue
		}
		dir := "sources/" + ref.SourceID
		if err = root.MkdirAll(dir, 0755); err != nil {
			return report, err
		}
		if err = writeExclusive(root, ref.Artifact, b); err != nil {
			return report, err
		}
		if err = syncDir(root, dir); err != nil {
			return report, err
		}
		if err = p.check("source-synced"); err != nil {
			return report, err
		}
	}
	if err = syncDir(root, "sources"); err != nil {
		return report, err
	}
	if err = syncDir(root, "."); err != nil {
		return report, err
	}
	staged := ".catalog-" + report.Generation + ".json"
	if err = writeExclusive(root, staged, catalog); err != nil {
		return report, err
	}
	if err = p.check("catalog-synced"); err != nil {
		return report, err
	}
	if _, _, err = Validate(catalog, func(name string) ([]byte, error) { return ReadDocument(root, name) }); err != nil {
		return report, err
	}
	if err = p.check("before-rename"); err != nil {
		return report, err
	}
	if err = root.Rename(staged, "catalog.json"); err != nil {
		return report, err
	}
	report.Published = true
	if err = p.check("after-rename"); err != nil {
		return report, err
	}
	if err = syncDir(root, "."); err != nil {
		return report, err
	}
	report.Durable = true
	return report, nil
}
