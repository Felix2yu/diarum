#!/bin/sh
# Mark directories to be ignored by iCloud FileProvider (macOS only).
# Called automatically after each install via postinstall.
# Exits silently on Linux/Docker where xattr is not available.

command -v xattr >/dev/null 2>&1 || exit 0

for dir in node_modules .nuxt .output .svelte-kit build dist .tmp; do
  [ -d "$dir" ] || continue
  xattr -w com.apple.fileprovider.ignore 1 "$dir" 2>/dev/null || true
done
