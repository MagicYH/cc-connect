#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# stop-mention-reminder.sh — Stop hook
#
# Reminds the bot that to send a message to a specific bot in
# the Feishu group chat, it MUST use the <at user_id="..."></at>
# tag. Plain text @name does NOT trigger the target bot.
# ============================================================

MSG='[cc-connect] Reminder: When you want to send a message to a specific bot in the group chat, you MUST use the Feishu <at> tag syntax (e.g. <at user_id="ou_xxx"></at>). Plain text like @name will NOT trigger the target bot — only the <at> tag routes the message correctly. Always use <at user_id="..."></at> followed by your message content.'

ESCAPED=$(echo "$MSG" | jq -Rs .)
echo "{\"hookSpecificOutput\":{\"hookEventName\":\"Stop\",\"additionalContext\":${ESCAPED}}}"
