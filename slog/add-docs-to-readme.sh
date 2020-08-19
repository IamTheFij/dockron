#! /bin/bash
set -e

slogdir=$(dirname "$0")
readme="$slogdir/Readme.md"

awk '/## Documentation/ {print ; exit} {print}' "$readme" > "$readme.tmp" && go doc -all slog | sed "s/^/    /;s/[ \t]*$//" >> "$readme.tmp"
mv "$readme.tmp" "$readme"
