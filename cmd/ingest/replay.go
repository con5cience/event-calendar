package main

import (
	"encoding/json"
	"event-calendar/internal/aeg"
	"event-calendar/internal/artifact"
	"event-calendar/internal/store"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// replay is intentionally local-only. The caller supplies an explicit fixture
// clock and existing test store; neither fetching nor scheduling is implemented.
func replay(args []string, out, errout io.Writer) int {
	return replaySnapshot(args, out, errout, "replay-aeg", aeg.Decode)
}

func replaySnapshot(args []string, out, errout io.Writer, command string, decode func(artifact.SourceConfig, []byte, time.Time) (artifact.Refresh, error)) int {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errout)
	dir := flags.String("store", "", "existing local test artifact store")
	config := flags.String("config", "", "source configuration YAML")
	snapshot := flags.String("snapshot", "", "local source JSON snapshot (no URLs)")
	at := flags.String("now", "", "explicit RFC3339 fixture clock")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	now, err := time.Parse(time.RFC3339Nano, *at)
	if err != nil || *dir == "" || *config == "" || *snapshot == "" || flags.NArg() != 0 {
		fmt.Fprintf(errout, "usage: ingest %s --store DIR --config YAML --snapshot JSON --now RFC3339\n", command)
		return 2
	}
	report := struct {
		store.Report
		Rejected []artifact.Rejection `json:"rejected"`
	}{Rejected: []artifact.Rejection{}}
	execute := func() error {
		b, err := readLocal(*config)
		if err != nil {
			return err
		}
		cfg, err := artifact.DecodeConfigYAML(b)
		if err != nil {
			return err
		}
		root, err := os.OpenRoot(*dir)
		if err != nil {
			return err
		}
		defer root.Close()
		expected := "none"
		var prior *artifact.Artifact
		// Read prior data and its generation together before normalization. The
		// publisher rejects a competing generation rather than overwriting its work.
		catalog, artifacts, err := store.Load(root)
		if err != nil {
			_, statErr := root.Stat("catalog.json")
			if !os.IsNotExist(statErr) || cfg.State != "new" {
				return err
			}
		} else {
			expected = catalog.Generation
			for i := range artifacts {
				if artifacts[i].Source.ID == cfg.Source.ID {
					prior = &artifacts[i]
					break
				}
			}
		}
		b, err = readLocal(*snapshot)
		if err != nil {
			return err
		}
		refresh, err := decode(cfg, b, now)
		if err != nil {
			return err
		}
		result, err := (&artifact.Reconciler{}).Reconcile(cfg, prior, refresh, refresh.Coverage.From)
		if err != nil {
			return err
		}
		report.Rejected = result.Rejected
		candidate, err := artifact.EncodeArtifact(result.Artifact)
		if err != nil {
			return err
		}
		report.Report, err = (&store.Publisher{}).Publish(*dir, expected, [][]byte{candidate})
		return err
	}
	err = execute()
	if err != nil {
		report.Error = err.Error()
	}
	if outputErr := json.NewEncoder(out).Encode(report); outputErr != nil {
		fmt.Fprintln(errout, outputErr)
		if report.Published {
			return 3
		}
		return 1
	}
	if err != nil {
		if report.Published {
			return 3
		}
		return 1
	}
	return 0
}

func readLocal(name string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return store.ReadDocument(root, filepath.Base(name))
}
