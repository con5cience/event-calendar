#!/usr/bin/env python3
"""Offline genre probe. Not ingest, and not a product change.

Reads the first stored performer. Asks MusicBrainz, Last.fm, Discogs, and
iTunes. Keeps one label only when two catalogs map to it and that label is
alone at the top. Misses are cached. Nothing is written onto events.
"""

import json
import os
import re
import sys
import time
import unicodedata
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter, defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from threading import Lock

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / ".artifacts" / "denver" / "catalog.json"
CACHE = ROOT / ".artifacts" / "genre-cache.json"
OUT = ROOT / ".artifacts" / "genre-consensus.json"
UA = "event-calendar-genre-probe/0.1 (local research; wstiern@gmail.com)"

LABELS = (
    "comedy",
    "classical",
    "jazz",
    "blues",
    "reggae",
    "latin",
    "hip-hop",
    "r&b",
    "soul",
    "electronic",
    "metal",
    "punk",
    "country",
    "folk",
    "indie",
    "pop",
    "rock",
    "experimental",
)
# First match wins. Compound catalog genres are listed before bare words.
TAG_LABEL = [
    ("stand-up comedy", "comedy"),
    ("stand up comedy", "comedy"),
    ("stand-up", "comedy"),
    ("comedy", "comedy"),
    ("singer-songwriter", "folk"),
    ("singer songwriter", "folk"),
    ("singer/songwriter", "folk"),
    ("folk, world, & country", None),
    ("funk / soul", None),
    ("stage & screen", None),
    ("hip-hop/rap", "hip-hop"),
    ("hip hop/rap", "hip-hop"),
    ("rhythm and blues", "r&b"),
    ("r&b/soul", "r&b"),
    ("contemporary r&b", "r&b"),
    ("neo-soul", "soul"),
    ("neo soul", "soul"),
    ("drum and bass", "electronic"),
    ("drum & bass", "electronic"),
    ("folk rock", "folk"),
    ("indie folk", "indie"),
    ("indie pop", "indie"),
    ("indie rock", "indie"),
    ("alt-country", "country"),
    ("outlaw country", "country"),
    ("contemporary country", "country"),
    ("country pop", "country"),
    ("blues rock", "blues"),
    ("pop punk", "punk"),
    ("post-punk", "punk"),
    ("post-hardcore", "punk"),
    ("hardcore punk", "punk"),
    ("alternative rock", "rock"),
    ("classic rock", "rock"),
    ("hard rock", "rock"),
    ("modern rock", "rock"),
    ("deathcore", "metal"),
    ("metalcore", "metal"),
    ("nu metal", "metal"),
    ("black metal", "metal"),
    ("death metal", "metal"),
    ("doom metal", "metal"),
    ("thrash metal", "metal"),
    ("heavy metal", "metal"),
    ("chamber music", "classical"),
    ("regional mexican", "latin"),
    ("latin pop", "latin"),
    ("southern hip hop", "hip-hop"),
    ("underground hip hop", "hip-hop"),
    ("electropop", "pop"),
    ("dance pop", "pop"),
    ("synth-pop", "pop"),
    ("synthpop", "pop"),
    ("art pop", "pop"),
    ("bedroom pop", "indie"),
    ("hip-hop", "hip-hop"),
    ("hip hop", "hip-hop"),
    ("r&b", "r&b"),
    ("rnb", "r&b"),
    ("soul", "soul"),
    ("motown", "soul"),
    ("dubstep", "electronic"),
    ("electronica", "electronic"),
    ("electronic", "electronic"),
    ("techno", "electronic"),
    ("trance", "electronic"),
    ("house", "electronic"),
    ("edm", "electronic"),
    ("jazz", "jazz"),
    ("blues", "blues"),
    ("reggae", "reggae"),
    ("dancehall", "reggae"),
    ("reggaeton", "latin"),
    ("cumbia", "latin"),
    ("bachata", "latin"),
    ("salsa", "latin"),
    ("latin", "latin"),
    ("latino", "latin"),
    ("metal", "metal"),
    ("hardcore", "punk"),
    ("punk", "punk"),
    ("emo", "punk"),
    ("country", "country"),
    ("americana", "folk"),
    ("bluegrass", "folk"),
    ("folk", "folk"),
    ("indie", "indie"),
    ("lo-fi", "indie"),
    ("pop", "pop"),
    ("rock", "rock"),
    ("rap", "hip-hop"),
    ("trap", "hip-hop"),
    ("classical", "classical"),
    ("opera", "classical"),
    ("experimental", "experimental"),
    ("avant-garde", "experimental"),
    ("dance", "electronic"),
    ("ska", "reggae"),
    ("dub", "reggae"),
]
IGNORE = {"seen live", "favorites", "spotify", "all", "under 2000 listeners", "alternative"}
mb_lock = Lock()
discogs_lock = Lock()
mb_next = 0.0
discogs_next = 0.0


def norm(value):
    value = unicodedata.normalize("NFKD", value or "").encode("ascii", "ignore").decode()
    value = value.lower().replace("&", " and ")
    value = re.sub(r"\([^)]*\)", " ", value)
    value = re.sub(r"[^a-z0-9]+", " ", value)
    return re.sub(r"\s+", " ", value).strip()


def label_for(tag):
    text = norm(tag)
    if not text or text in IGNORE:
        return None
    for needle, label in TAG_LABEL:
        if norm(needle) in text:
            return label
    return None


def labels_for(tags):
    found = []
    for tag in tags:
        label = label_for(tag)
        if label and label not in found:
            found.append(label)
    return found


def decide(votes):
    counts = Counter()
    for labels in votes.values():
        for label in set(labels):
            counts[label] += 1
    agreed = [(label, count) for label, count in counts.items() if count >= 2]
    agreed.sort(key=lambda item: (-item[1], LABELS.index(item[0])))
    if not agreed or (len(agreed) > 1 and agreed[0][1] == agreed[1][1]):
        return None, counts
    return agreed[0][0], counts


def get(url, headers, timeout=25):
    request = urllib.request.Request(url, headers=headers)
    for attempt in range(4):
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            if error.code in {429, 503} and attempt < 3:
                time.sleep(2 + attempt * 3)
                continue
            return {"_error": f"HTTP {error.code}"}
        except Exception as error:
            if attempt < 3:
                time.sleep(1 + attempt)
                continue
            return {"_error": str(error)}
    return {"_error": "failed"}


def limited(lock, slot, url, headers):
    global mb_next, discogs_next
    with lock:
        nxt = mb_next if slot == "mb" else discogs_next
        wait = nxt - time.monotonic()
        if wait > 0:
            time.sleep(wait)
        if slot == "mb":
            mb_next = time.monotonic() + 1.1
        else:
            discogs_next = time.monotonic() + 1.05
    return get(url, headers)


def unique_named(items, name, field):
    wanted = norm(name)
    hits = [item for item in items if norm(item.get(field, "")) == wanted]
    return hits[0] if len(hits) == 1 else None


def musicbrainz(name):
    query = urllib.parse.quote(f'artist:"{name}"')
    found = limited(
        mb_lock,
        "mb",
        f"https://musicbrainz.org/ws/2/artist/?query={query}&fmt=json&limit=5",
        {"User-Agent": UA, "Accept": "application/json"},
    )
    if found.get("_error"):
        return {"error": found["_error"], "labels": []}
    artist = unique_named(found.get("artists") or [], name, "name")
    if not artist:
        return {"matched": False, "labels": []}
    detail = limited(
        mb_lock,
        "mb",
        f"https://musicbrainz.org/ws/2/artist/{artist['id']}?inc=genres+tags&fmt=json",
        {"User-Agent": UA, "Accept": "application/json"},
    )
    if detail.get("_error"):
        return {"matched": True, "name": artist["name"], "error": detail["_error"], "labels": []}
    tags = [item.get("name", "") for item in (detail.get("genres") or [])]
    if not tags:
        tags = [
            item.get("name", "")
            for item in (detail.get("tags") or [])
            if item.get("count", 0) > 0
        ]
    return {"matched": True, "name": artist["name"], "tags": tags[:12], "labels": labels_for(tags)}


def lastfm(name, key):
    query = urllib.parse.urlencode(
        {
            "method": "artist.getinfo",
            "artist": name,
            "api_key": key,
            "format": "json",
            "autocorrect": "0",
        }
    )
    data = get(f"https://ws.audioscrobbler.com/2.0/?{query}", {"User-Agent": UA})
    if data.get("_error") or data.get("error"):
        return {"error": data.get("message") or data.get("_error"), "labels": []}
    artist = data.get("artist") or {}
    if norm(artist.get("name", "")) != norm(name):
        return {"matched": False, "labels": [], "name": artist.get("name")}
    raw = ((artist.get("tags") or {}).get("tag")) or []
    tags = [item.get("name", "") for item in raw if isinstance(item, dict)]
    return {"matched": True, "name": artist.get("name"), "tags": tags[:12], "labels": labels_for(tags)}


def discogs(name, token):
    headers = {"Authorization": f"Discogs token={token}", "User-Agent": UA}
    query = urllib.parse.urlencode({"q": name, "type": "artist", "per_page": 5})
    found = limited(
        discogs_lock,
        "discogs",
        f"https://api.discogs.com/database/search?{query}",
        headers,
    )
    if found.get("_error"):
        return {"error": found["_error"], "labels": []}
    artist = unique_named(found.get("results") or [], name, "title")
    if not artist:
        return {"matched": False, "labels": []}
    releases = limited(
        discogs_lock,
        "discogs",
        f"https://api.discogs.com/artists/{artist['id']}/releases?per_page=5",
        headers,
    )
    if releases.get("_error"):
        return {"matched": True, "name": artist.get("title"), "error": releases["_error"], "labels": []}
    main = next(
        (item for item in releases.get("releases") or [] if item.get("role") == "Main"),
        None,
    )
    if not main:
        return {"matched": True, "name": artist.get("title"), "labels": []}
    path = "masters" if main.get("type") == "master" else "releases"
    detail = limited(
        discogs_lock,
        "discogs",
        f"https://api.discogs.com/{path}/{main['id']}",
        headers,
    )
    if detail.get("_error"):
        return {"matched": True, "name": artist.get("title"), "error": detail["_error"], "labels": []}
    styles = detail.get("styles") or []
    genres = detail.get("genres") or []
    tags = styles or genres
    return {
        "matched": True,
        "name": artist.get("title"),
        "tags": tags[:12],
        "labels": labels_for(tags),
    }


def itunes(name):
    query = urllib.parse.urlencode({"term": name, "entity": "musicArtist", "limit": 5})
    data = get(f"https://itunes.apple.com/search?{query}", {"User-Agent": UA})
    if data.get("_error"):
        return {"error": data["_error"], "labels": []}
    artist = unique_named(data.get("results") or [], name, "artistName")
    if not artist:
        return {"matched": False, "labels": []}
    tag = artist.get("primaryGenreName") or ""
    return {
        "matched": True,
        "name": artist.get("artistName"),
        "tags": [tag] if tag else [],
        "labels": labels_for([tag]),
    }


def lookup(name, lastfm_key, discogs_token):
    with ThreadPoolExecutor(max_workers=4) as pool:
        pending = {
            pool.submit(musicbrainz, name): "musicbrainz",
            pool.submit(lastfm, name, lastfm_key): "lastfm",
            pool.submit(discogs, name, discogs_token): "discogs",
            pool.submit(itunes, name): "itunes",
        }
        sources = {}
        for future in as_completed(pending):
            sources[pending[future]] = future.result()
    votes = {key: value.get("labels") or [] for key, value in sources.items()}
    label, counts = decide(votes)
    agreed = sorted(key for key, labels in votes.items() if label and label in labels)
    return {"label": label, "agreed": agreed, "counts": dict(counts), "sources": sources}


def artists_from_catalog():
    catalog = json.loads(CATALOG.read_text())
    found = {}
    for source in catalog["sources"]:
        artifact = json.loads((CATALOG.parent / source["artifact"]).read_text())
        for event in artifact["events"]:
            performers = event.get("performers") or []
            if not performers or not performers[0].strip():
                continue
            original = performers[0].strip()
            key = norm(original)
            if not key:
                continue
            row = found.setdefault(key, {"name": original, "events": 0, "sources": set()})
            row["events"] += 1
            row["sources"].add(source["source_id"])
    return found


def main():
    missing = [key for key in ("LASTFM_API_KEY", "DISCOGS_API_TOKEN") if not os.environ.get(key)]
    if missing:
        raise SystemExit(f"Missing env: {', '.join(missing)}")
    limit = int(sys.argv[1]) if len(sys.argv) > 1 else 0
    cache = json.loads(CACHE.read_text()) if CACHE.exists() else {}
    rows = artists_from_catalog()
    pending = [key for key in rows if key not in cache]
    if limit:
        pending = pending[:limit]
    print(f"artists={len(rows)} cached={len(rows)-len([k for k in rows if k not in cache])} to_query={len(pending)}", flush=True)
    for done, key in enumerate(pending, 1):
        name = rows[key]["name"]
        result = None
        for attempt in range(3):
            try:
                result = lookup(name, os.environ["LASTFM_API_KEY"], os.environ["DISCOGS_API_TOKEN"])
            except Exception as error:
                print(f"retry {name}: {error}", flush=True)
                time.sleep(2 + attempt)
                continue
            if not any(
                (result.get("sources") or {}).get(src, {}).get("error")
                for src in ("musicbrainz", "lastfm", "discogs", "itunes")
            ):
                break
            print(f"retry {name}: source error", flush=True)
            time.sleep(2 + attempt)
            result = None
        if not result:
            continue
        cache[key] = result
        if done % 20 == 0 or done == len(pending):
            CACHE.write_text(json.dumps(cache))
            print(f"queried {done}/{len(pending)} {name} -> {cache[key].get('label')}", flush=True)
    report = []
    for key, row in sorted(rows.items(), key=lambda item: item[1]["name"].lower()):
        cached = cache.get(key) or {}
        report.append(
            {
                "name": row["name"],
                "events": row["events"],
                "sources": sorted(row["sources"]),
                "label": cached.get("label"),
                "agreed": cached.get("agreed") or [],
                "counts": cached.get("counts") or {},
            }
        )
    OUT.write_text(json.dumps(report, indent=2) + "\n")
    labeled = [row for row in report if row["label"]]
    print(
        f"labeled={len(labeled)} untagged={len(report)-len(labeled)} of {len(report)}",
        flush=True,
    )
    by_label = defaultdict(list)
    for row in labeled:
        by_label[row["label"]].append(row["name"])
    for label in LABELS:
        names = by_label.get(label) or []
        if names:
            print(f"\n{label} ({len(names)})")
            print(", ".join(names))


if __name__ == "__main__":
    main()
