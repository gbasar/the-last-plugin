---
description: Download one month of transactions
argument-hint: "[YYYY-MM, default last month]"
---
Download the transactions for `$1` (if empty: last month) from <source: bank
export, API or database> into `<download-dir>/$1.csv`.

- If the file is already there, say so and reuse it; don't download twice.
- Report: the file path, the row count, the first and last dates in it.
- On failure, stop and show the error. Don't retry with another method.
