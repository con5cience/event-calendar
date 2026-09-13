# Synthetic AEG fixtures

These are authored test inputs, not saved provider responses. Each JSON file contains
one invented event with the same observed field structure. `example.com` ticket links
and provider ID `1001` are placeholders. Do not publish these as real listings.

The YAML files configure the five supported venues: Gothic, Mission, Bluebird,
Ogden, and Fiddler's Green. `state: new` is correct
only before that source's first publication. For a subsequent local replay, copy the
configuration into your test input directory and set `state: established`. Do not
change the checked-in fixture configuration during tests.

All five YAML files additionally contain real, manually reviewed admission rules
from their official policy pages, with a September 9, 2026 review date. JSON events
remain synthetic. Bluebird, Gothic, Mission, and Ogden cover All ages, 16+, 18+,
and 21+ only; missing or 13+ restrictions have no inferred permission. Fiddler's
Green covers All ages only. Its synthetic event remains 16+ to verify that a
restricted event does not inherit the all-ages rule. The container replay checks
that the other four venues match With adult. The opt-in live admission browser
test also exercises Fiddler's Green's real all-ages records.

Tests mutate copies to exercise malformed metadata, cap/identity failures, invalid
updates, missing records, sparse fields, and the date window. All network access is
outside this fixture workflow. See `docs/adapters/0002-aeg-json-feeds.md` for the
mapping contract and unresolved live-access gate.
