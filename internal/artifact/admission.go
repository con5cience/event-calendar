package artifact

import "fmt"

func validateAdmission(a *AdultAdmission) error {
	if a == nil {
		return nil
	}
	last := -1
	for _, r := range a.Ranges {
		if r.MinAge > r.MaxAge || r.MinAge <= last {
			return fmt.Errorf("admission ranges must be ordered and disjoint")
		}
		last = r.MaxAge
	}
	return nil
}

// EnrichAdmission adds reviewed conditions only for on-site, known restrictions.
// Explicit event admission information and configured policy overrides win.
func EnrichAdmission(cfg SourceConfig, upstreamID string, data EventData) EventData {
	if data.Venue != nil {
		return data
	}
	for _, o := range cfg.Overrides {
		if o.Match.UpstreamID != upstreamID || o.Match.Date != data.Date || o.Match.VenueKey != cfg.Venue.Key {
			continue
		}
		if _, ok := o.Set["admission_policy"]; ok {
			return data
		}
		for _, field := range o.Remove {
			if field == "admission_policy" {
				return data
			}
		}
	}
	policy := data.AdmissionPolicy
	if policy == nil {
		policy = cfg.Venue.AdmissionPolicy
	}
	if policy == nil || policy.WithAdult != nil {
		return data
	}
	rule, ok := cfg.AdmissionRules[policy.Category]
	if !ok {
		return data
	}
	copy := *policy
	copy.WithAdult = &rule
	data.AdmissionPolicy = &copy
	return data
}
