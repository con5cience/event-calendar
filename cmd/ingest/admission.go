package main

import (
	"encoding/json"
	"event-calendar/internal/artifact"
	"event-calendar/internal/store"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// Enrich an existing source without refreshing upstream, reconciling absence,
// or changing occurrence identities and last-known listing state.
func enrichAdmission(args []string, out, errout io.Writer) int {
	flags := flag.NewFlagSet("enrich-admission", flag.ContinueOnError)
	flags.SetOutput(errout)
	dir := flags.String("store", "", "existing artifact store")
	config := flags.String("config", "", "established source configuration YAML")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *dir == "" || *config == "" || flags.NArg() != 0 {
		fmt.Fprintln(errout, "usage: ingest enrich-admission --store DIR --config YAML")
		return 2
	}
	var report store.Report
	execute := func() error {
		b, err := readLocal(*config)
		if err != nil {
			return err
		}
		cfg, err := artifact.DecodeConfigYAML(b)
		if err != nil {
			return err
		}
		if cfg.State != "established" || len(cfg.AdmissionRules) == 0 {
			return fmt.Errorf("enrichment requires established source and reviewed rules")
		}
		root, err := os.OpenRoot(*dir)
		if err != nil {
			return err
		}
		defer root.Close()
		catalog, artifacts, err := store.Load(root)
		if err != nil {
			return err
		}
		for _, a := range artifacts {
			if a.Source.ID != cfg.Source.ID {
				continue
			}
			if a.Source.Adapter != cfg.Source.Adapter || a.Venue.Key != cfg.Venue.Key || a.Venue.Website != cfg.Venue.Website {
				return fmt.Errorf("configuration does not match stored source venue")
			}
			for i := range a.Events {
				a.Events[i].EventData = artifact.EnrichAdmission(cfg, a.Events[i].UpstreamID, a.Events[i].EventData)
			}
			a.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
			candidate, err := artifact.EncodeArtifact(a)
			if err != nil {
				return err
			}
			report, err = (&store.Publisher{}).Publish(*dir, catalog.Generation, [][]byte{candidate})
			return err
		}
		return fmt.Errorf("source is not published")
	}
	err := execute()
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
