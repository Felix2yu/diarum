#!/bin/bash
# Mark directories to be ignored by iCloud FileProvider.
# Called automatically after each install via postinstall.

DIRS=(node_modules .nuxt .output .svelte-kit build dist .tmp)

for dir in "${DIRS[@]}"; do
  [ -d "$dir" ] || continue
  xattr -w com.apple.fileprovider.ignore 1 "$dir" 2>/dev/null
done
