# ADR 0024: Genre enrichment research

- Status: Proposed. Research only.
- Date: 2026-09-17
- Basis: Product interview, round 1, and the same-day source and catalog probes.
- Does not amend [ADR 0014](0014-product-and-delivery-contract.md). Classification remains deferred there, and external enrichment remains outside the current phase, until this ADR is accepted.

This ADR does not authorize a schema change, ingest change, filter, search change, database, or deployment. A later build can be tested. It cannot change the product until the research, the built probe, its tests, and an explicit acceptance are recorded here.

## Accepted interview decisions

1. This is a new ADR. It is a research track. It cannot change the product until research is done, a thing is built, it is tested, and it is accepted.
2. Nothing is public until coverage is reviewed. The intended control, after acceptance, is a dropdown beside Venue. The label must also match text search. That describes the target, not permission to add the control now.
3. A wrong tag is worse than no tag. Disagreement or a weak artist match stays untagged.
4. Store the genre on the event if that can be done without an artist database.
5. Do not add a database unless evidence shows it is required. Separately: a venue refresh must not call an external catalog, and a lookup failure must not fail the calendar. Catalog lookups, if built, are an offline job. Ingest may read only a local result it already has.
6. First catalogs, if a probe is built: MusicBrainz for identity, Last.fm and Discogs for tags, iTunes as one coarse vote. Spotify is out unless its artist genre field returns. A genre the venue feed already supplies is authoritative and is not overwritten.
7. Coverage is best effort. There is no numeric floor. The review gate in decision 2 still applies: best effort is not permission to publish the control.

## What the probes found

These are observations, not accepted rules.

Upstream musical genre is sparse. Of 1,620 events in the local Denver catalog on September 17, about 13% had a specific musical genre in a field the existing captures already store. That came from Marquis, Summit, and Fillmore Ticketmaster `genre` values, 26 Paramount events in the Music segment, and about 25 Ball Arena listing cards whose `data-subgenre` was musical rather than NHL, NBA, or an ice show. Red Rocks `Concert` / `Fitness`, Meow Wolf audience tags, and empty RHP `tags` arrays are not musical genres. AEG event objects in the stored captures have no genre field. HoldMyTicket feeds that were captured have no `CATEGORIES`. Hi-Dive, Herb's, Black Box, Levitt, and Black Buzzard mention genre in prose only.

Stored performers do not close that gap. 650 events had a performer, across 629 unique first names. Some of those strings are tour titles, not artists. The rooms without a performer are mostly the rooms without an upstream genre.

A live three-catalog lookup is a poor refresh step. Spotify's artist API no longer returns genres or popularity. MusicBrainz can match an artist and still return no tags. Last.fm returns a usable tag list and also noise. Discogs styles are on releases, not the search hit, and the top artist hit can be the wrong act: Bahamas the band versus an unrelated Bahamas. iTunes returns one coarse `primaryGenreName` and the same kind of collision. Requiring two catalogs to agree therefore labels little. Relaxing that rule labels the wrong act.

An uncommitted probe, `scripts/genre-consensus.py`, was started and stopped. It is not an accepted tool, not wired to ingest, and not evidence that a file cache is sufficient.

## Constraint, not a decision

Decisions 4 and 5 prefer an event genre and no database. The probe shows why a refresh-time lookup does not satisfy decision 5's other half: ingest must not call these catalogs, and a weak match must stay blank. A local file of resolved names may be enough. A database may be required for identity, rate limits, and review. Neither is accepted. Do not add one to get ahead of that evidence.

## Open questions

- Can a reviewed file cache, keyed by normalized performer name, satisfy decisions 4 and 5, or does artist collision force a database?
- Who reviews coverage before decision 2 allows the dropdown, and what artifact do they inspect?
- Does "venue-supplied" include Ticketmaster and Ball Arena labels already in captured payloads, or only text the venue wrote on its own page?
- Are comedy, sports, and film labels in the same dropdown as musical genre, or out of scope?
- What exact match rule counts as weak: multiple same-name hits, a release-level Discogs style, an iTunes-only vote, or a MusicBrainz match with no tags?

## Not authorized

- Publishing genre on events, or reading genre in the app.
- Adding the dropdown or search field.
- Calling MusicBrainz, Last.fm, Discogs, iTunes, or Spotify from ingest or refresh.
- Overwriting a venue-supplied genre.
- Treating a single catalog tag, a prose mention, or an empty tag list as a label.
- Adding a database, cache format, or adapter change under this ADR.
