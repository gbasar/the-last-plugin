---
description: Download a month of transactions and replay them
argument-hint: "[YYYY-MM, default last month]"
---
Replay the transactions for `$1` (if empty: last month). Do the steps in
order and stop at the first that fails.

1. /download-transactions $1
2. Replay `<download-dir>/$1.csv` against <target: staging, a local DB> with
   <replay command>.
3. Compare the replayed totals with the file's totals. Report: rows replayed,
   rows rejected (with the first 5 reasons), and whether the totals match.
