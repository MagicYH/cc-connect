---
name: kill-ai-rep
description: Use when ai-rep processes need to be terminated, or when asked to kill, stop, or clean up hanging ai-rep instances
---

# Kill ai-rep

Terminate all running `ai-rep` processes.

## Command

```bash
ps -ef | grep 'npm exec ai-rep' | awk '{print $2}' | xargs -I {} kill -9 {}
```

If no processes are found, the command completes silently.

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Using `kill` without `-9` | ai-rep may not respond to SIGTERM; use `-9` (SIGKILL) |
| Killing too broadly (e.g. `grep ai-rep` matches grep itself) | The pipe through `grep` already handles this — `ps` output doesn't include the grep command |
