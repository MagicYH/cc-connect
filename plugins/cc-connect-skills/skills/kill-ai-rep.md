---
name: kill-ai-rep
description: Kill all running ai-rep processes (npm exec ai-rep)
---

Kill all running `ai-rep` processes by finding their PIDs and force-terminating them.

## Command

```bash
ps -ef | grep 'npm exec ai-rep' | awk '{print $2}' | xargs -I {} kill -9 {}
```

## What it does

1. Lists all processes (`ps -ef`)
2. Filters for lines containing `npm exec ai-rep` (`grep`)
3. Extracts the PID (column 2) (`awk`)
4. Force-kills each matching process (`kill -9`)

## Usage

When the user asks to kill, stop, or terminate ai-rep processes, run the command above directly via Bash. If no processes are found, the command completes silently — no error is reported.
