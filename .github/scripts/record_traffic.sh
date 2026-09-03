#!/usr/bin/env bash
# Snapshots GitHub's 14-day clone/view traffic into a durable history file.
#
# GitHub only retains traffic data for 14 days, so this script is meant to
# run on a schedule (see .github/workflows/traffic.yml) and append each
# day's counts to traffic-history.json, deduping by date.
set -euo pipefail

REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY not set}"
HISTORY_FILE="traffic-history.json"

if [[ ! -f "$HISTORY_FILE" ]]; then
  echo '{"clones": [], "views": []}' >"$HISTORY_FILE"
fi

clones_json=$(gh api "repos/${REPO}/traffic/clones" --jq '.clones')
views_json=$(gh api "repos/${REPO}/traffic/views" --jq '.views')

tmp=$(mktemp)
jq \
  --argjson new_clones "$clones_json" \
  --argjson new_views "$views_json" \
  '
  def merge_series(existing; incoming):
    (existing + incoming)
    | group_by(.timestamp)
    | map(max_by(.count));

  .clones = merge_series(.clones; $new_clones) | .clones |= sort_by(.timestamp)
  | .views = merge_series(.views; $new_views) | .views |= sort_by(.timestamp)
  ' "$HISTORY_FILE" >"$tmp"
mv "$tmp" "$HISTORY_FILE"

echo "Updated $HISTORY_FILE"
