package hmt

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var calendarProperty = regexp.MustCompile(`^[A-Za-z0-9-]+[;:]`)

// stripFederalDescriptions discards unused descriptions from the observed
// Federal layout only. Raw snapshots remain untouched. CREATED must terminate
// each block; other property-looking lines fail closed instead of being lost.
// The standard parser still validates all retained calendar fields.
func stripFederalDescriptions(raw string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	inEvent, inDescription, seenDescription := false, false, false
	for _, line := range lines {
		if inDescription {
			if strings.HasPrefix(line, "CREATED:") {
				value := strings.TrimPrefix(line, "CREATED:")
				t, err := time.Parse("20060102T150405Z", value)
				if err != nil || t.Format("20060102T150405Z") != value {
					return "", fmt.Errorf("invalid Federal description boundary")
				}
				inDescription = false
				out = append(out, line)
			} else if calendarProperty.MatchString(line) {
				return "", fmt.Errorf("ambiguous Federal description boundary")
			}
			continue
		}
		if strings.HasPrefix(line, "DESCRIPTION:") {
			if !inEvent || seenDescription {
				return "", fmt.Errorf("unexpected Federal description")
			}
			inDescription, seenDescription = true, true
			continue
		}
		if line == "BEGIN:VEVENT" {
			if inEvent {
				return "", fmt.Errorf("nested Federal event")
			}
			inEvent, seenDescription = true, false
		} else if line == "END:VEVENT" {
			inEvent = false
		}
		out = append(out, line)
	}
	if inDescription {
		return "", fmt.Errorf("unterminated Federal description")
	}
	return strings.Join(out, "\r\n"), nil
}
