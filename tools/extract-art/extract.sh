#!/bin/sh
# Regenerates internal/aquarium/art/art.json from the Perl asciiquarium.
#
#   tools/extract-art/extract.sh path/to/asciiquarium
#
# Needs perl with JSON::PP (core) and python3. Term::Animation and Curses are
# NOT needed: dump.pl stubs them out and records what the Perl code would
# have drawn.
set -eu
src=${1:?usage: $0 path/to/asciiquarium}
here=$(dirname "$0")
perl "$here/dump.pl" "$src" | python3 "$here/shape.py" > "$here/../../internal/aquarium/art/art.json"
echo "wrote internal/aquarium/art/art.json"
