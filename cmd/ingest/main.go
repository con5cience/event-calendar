package main

import (
	"encoding/json"
	"event-calendar/internal/afton"
	"event-calendar/internal/blackbox"
	"event-calendar/internal/buzzard"
	"event-calendar/internal/clique"
	"event-calendar/internal/herbs"
	"event-calendar/internal/hmt"
	"event-calendar/internal/kse"
	"event-calendar/internal/livenation"
	"event-calendar/internal/meowwolf"
	"event-calendar/internal/ophelias"
	"event-calendar/internal/plot"
	"event-calendar/internal/rhp"
	"event-calendar/internal/store"
	"event-calendar/internal/venuepilot"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func run(args []string, out, errout io.Writer) int {
	if len(args) > 0 && args[0] == "replay-afton" {
		return replaySnapshot(args[1:], out, errout, "replay-afton", afton.Decode)
	}
	if len(args) > 0 && args[0] == "replay-herbs" {
		return replaySnapshot(args[1:], out, errout, "replay-herbs", herbs.Decode)
	}
	if len(args) > 0 && args[0] == "replay-buzzard" {
		return replaySnapshot(args[1:], out, errout, "replay-buzzard", buzzard.Decode)
	}
	if len(args) > 0 && args[0] == "replay-ophelias" {
		return replaySnapshot(args[1:], out, errout, "replay-ophelias", ophelias.Decode)
	}
	if len(args) > 0 && args[0] == "replay-meowwolf" {
		return replaySnapshot(args[1:], out, errout, "replay-meowwolf", meowwolf.Decode)
	}
	if len(args) > 0 && args[0] == "replay-venuepilot" {
		return replaySnapshot(args[1:], out, errout, "replay-venuepilot", venuepilot.Decode)
	}
	if len(args) > 0 && args[0] == "replay-blackbox" {
		return replaySnapshot(args[1:], out, errout, "replay-blackbox", blackbox.Decode)
	}
	if len(args) > 0 && args[0] == "replay-kse-calendar" {
		return replaySnapshot(args[1:], out, errout, "replay-kse-calendar", kse.DecodeBall)
	}
	if len(args) > 0 && args[0] == "replay-livenation" {
		return replaySnapshot(args[1:], out, errout, "replay-livenation", livenation.Decode)
	}
	if len(args) > 0 && args[0] == "replay-kse" {
		return replaySnapshot(args[1:], out, errout, "replay-kse", kse.Decode)
	}
	if len(args) > 0 && args[0] == "replay-plot" {
		return replaySnapshot(args[1:], out, errout, "replay-plot", plot.Decode)
	}
	if len(args) > 0 && args[0] == "replay-clique" {
		return replaySnapshot(args[1:], out, errout, "replay-clique", clique.Decode)
	}
	if len(args) > 0 && args[0] == "replay-rhp" {
		return replaySnapshot(args[1:], out, errout, "replay-rhp", rhp.Decode)
	}
	if len(args) > 0 && args[0] == "enrich-admission" {
		return enrichAdmission(args[1:], out, errout)
	}
	if len(args) > 0 && args[0] == "replay-aeg" {
		return replay(args[1:], out, errout)
	}
	if len(args) > 0 && args[0] == "replay-hmt" {
		return replaySnapshot(args[1:], out, errout, "replay-hmt", hmt.Decode)
	}
	usage := func() {
		fmt.Fprintln(errout, "usage: ingest publish --store DIR --expect GENERATION|none ARTIFACT.json [...]")
	}
	if len(args) == 0 || args[0] != "publish" {
		usage()
		return 2
	}
	flags := flag.NewFlagSet("publish", flag.ContinueOnError)
	flags.SetOutput(errout)
	dir := flags.String("store", "", "existing local artifact-store directory")
	expected := flags.String("expect", "", "current catalog generation, or none for an absent catalog")
	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *dir == "" || *expected == "" || flags.NArg() == 0 {
		usage()
		return 2
	}
	inputs := [][]byte{}
	total := 0
	for _, name := range flags.Args() {
		root, err := os.OpenRoot(filepath.Dir(name))
		if err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
		b, err := store.ReadDocument(root, filepath.Base(name))
		root.Close()
		if err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
		total += len(b)
		if total > store.MaxGenerationBytes {
			fmt.Fprintln(errout, "candidates exceed 64 MiB")
			return 1
		}
		inputs = append(inputs, b)
	}
	report, err := (&store.Publisher{}).Publish(*dir, *expected, inputs)
	if err != nil {
		report.Error = err.Error()
	}
	if encodeErr := json.NewEncoder(out).Encode(report); encodeErr != nil {
		fmt.Fprintln(errout, encodeErr)
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
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
