#!/bin/bash
set -e
PAT="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"
cd /mnt/e/360MoveData/Users/Administrator/Desktop/anytls/vasmax

# Add all changes and commit
git add -A
git status --short

# Check if there's anything to commit
if git diff --cached --quiet; then
  echo "Nothing staged, checking untracked..."
  git status
else
  git commit -m "fix: v2.2.0 - critical bug fixes across subscription, stats, compilation and UI"
fi

git remote set-url origin "https://${PAT}@github.com/jungann2/vasmax-dev.git"
git push origin main
git remote set-url origin "https://github.com/jungann2/vasmax-dev.git"
echo "=== vasmax-dev pushed ==="
