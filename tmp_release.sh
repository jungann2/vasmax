#!/bin/bash
set -e
PAT="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"

cd /mnt/e/360MoveData/Users/Administrator/Desktop/anytls/vasmax

git add -A
git commit -m "fix: v2.2.0 - critical bug fixes across subscription, stats, compilation and UI

- Fix subscription links: hardcoded port 443, wrong WS path, empty nodomain address
- Fix salt mismatch causing subscription 404
- Fix Xray Stats missing user-level traffic config (all traffic reporting was zero)
- Fix compilation failure from 3 legacy duplicate declaration files
- Fix remote subscription URL construction (was completely broken)
- Fix rollback not restarting services after restore
- Fix statEntry.Value type mismatch breaking monitor traffic display
- Fix port menu numbering conflict
- Add user speed/device limit editing in account management
- Add subscription management usage notes
- Improve fake site menu description"

git remote set-url origin "https://${PAT}@github.com/jungann2/vasmax-dev.git"
git push origin main
git remote set-url origin "https://github.com/jungann2/vasmax-dev.git"
echo "=== vasmax-dev pushed ==="
