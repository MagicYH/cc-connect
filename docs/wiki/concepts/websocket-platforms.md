# WebSocket Platform Connections

Most platforms connect via long-lived WebSocket clients: Feishu (official SDK), WeCom, Weibo, WPS Xiezuo, QQ Bot, DingTalk Stream, Slack Socket Mode, Discord Gateway, QQ (OneBot/NapCat). Two platforms use HTTP-only: Telegram (long-polling) and LINE (webhook callback + REST). WeCom supports both WebSocket and HTTP callback modes.

**SDKs:** `larksuite/oapi-sdk-go/v3` (Feishu), `open-dingtalk/dingtalk-stream-sdk-go` (DingTalk), `gorilla/websocket` (most others), `bwmarrin/discordgo` (Discord), `slack-go/slack` (Slack), `go-telegram/bot` (Telegram), `line/line-bot-sdk-go/v8` (LINE)
