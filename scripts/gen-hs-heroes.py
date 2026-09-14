#!/usr/bin/env python3
"""Regenerate web/src/lib/hs-heroes.json, the Battlegrounds hero names.

Run it when Blizzard adds heroes; until then an unknown identifier shows as
itself on the game page, which degrades one label and not the page.

    python3 scripts/gen-hs-heroes.py

The public catalogue weighs ten megabytes per language, which is why the result
is vendored rather than fetched at runtime. Skins are left out: the server folds
them onto the hero they dress before the page ever sees them.
"""
import json
import re
import urllib.request

URL = "https://api.hearthstonejson.com/v1/latest/{}/cards.json"
OUT = "web/src/lib/hs-heroes.json"


def fetch(locale):
    # The catalogue refuses urllib's default agent with a 403.
    req = urllib.request.Request(URL.format(locale),
                                 headers={"User-Agent": "kfire-server/hs-heroes"})
    with urllib.request.urlopen(req) as r:
        return {c["id"]: c for c in json.load(r)}


def main():
    fr, en = fetch("frFR"), fetch("enUS")
    heroes = {}
    for cid, card in fr.items():
        if card.get("type") != "HERO" or re.search(r"_SKIN_\w+$", cid):
            continue
        # The flag alone misses heroes Blizzard has retired, which members may
        # still have played; the identifier shapes catch those.
        if (card.get("battlegroundsHero")
                or cid.startswith("TB_BaconShop_HERO_")
                or re.match(r"^BG\d+_HERO_\d+$", cid)):
            heroes[cid] = [card.get("name"), en.get(cid, {}).get("name")]
    with open(OUT, "w", encoding="utf-8") as f:
        json.dump(dict(sorted(heroes.items())), f, ensure_ascii=False, indent=1)
        f.write("\n")
    print(f"{len(heroes)} heroes written to {OUT}")


if __name__ == "__main__":
    main()
