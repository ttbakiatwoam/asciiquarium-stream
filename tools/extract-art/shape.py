"""Reshapes dump.pl output into the art.json layout the Go code embeds."""
import json
import sys

d = json.load(sys.stdin)
out = {}
for k, v in d.items():
    if k in ("add_new_fish", "add_old_fish"):
        out[k.replace("add_", "")] = [{"shape": v[i], "color": v[i + 1]} for i in range(0, len(v), 2)]
        continue
    name, mode = k.split("/")
    ents = [{kk: e[kk] for kk in ("shape", "color", "default_color") if kk in e} for e in v]
    out.setdefault(name.replace("add_", ""), {})[mode] = ents
json.dump(out, sys.stdout, indent=1, sort_keys=True)
