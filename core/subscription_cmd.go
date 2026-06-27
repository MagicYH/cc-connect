package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func (e *Engine) cmdSubscribe(p Platform, msg *Message, args []string) {
	if e.subscriptionManager == nil {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubNotAvailable))
		return
	}

	if len(args) == 0 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
		return
	}

	sub := matchSubCommand(strings.ToLower(args[0]), []string{
		"add", "list", "show", "enable", "disable", "del", "delete", "rm", "remove", "help",
	})
	switch sub {
	case "add":
		e.cmdSubscribeAdd(p, msg, args[1:])
	case "list":
		e.cmdSubscribeList(p, msg, args[1:])
	case "show":
		e.cmdSubscribeShow(p, msg, args[1:])
	case "enable":
		e.cmdSubscribeEnable(p, msg, args[1:])
	case "disable":
		e.cmdSubscribeDisable(p, msg, args[1:])
	case "del", "delete", "rm", "remove":
		e.cmdSubscribeDel(p, msg, args[1:])
	case "help":
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
	default:
		// Default: treat as "add" with the args as the filter
		e.cmdSubscribeAdd(p, msg, args)
	}
}

func (e *Engine) cmdSubscribeAdd(p Platform, msg *Message, args []string) {
	if len(args) == 0 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
		return
	}

	// Parse flags: first non-flag arg is the filter
	var (
		filter        string
		excludeFilter string
		prompt        string
		interval      string
	)

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--exclude", "-e":
			i++
			if i < len(args) {
				excludeFilter = args[i]
			}
		case "--prompt", "-p":
			i++
			if i < len(args) {
				prompt = args[i]
			}
		case "--interval", "-i":
			i++
			if i < len(args) {
				interval = args[i]
			}
		default:
			if filter == "" {
				filter = args[i]
			}
		}
		i++
	}

	if filter == "" {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
		return
	}
	if prompt == "" {
		prompt = DefaultSubscriptionPrompt
	}
	if interval == "" {
		interval = DefaultSubscriptionInterval
	}

	chatID := extractChannelID(msg.SessionKey)
	platformName, _, _ := parseSessionKeyParts(msg.SessionKey)

	sub := &Subscription{
		ID:            GenerateSubscriptionID(),
		Project:       e.name,
		ChatID:        chatID,
		ChatName:      msg.ChatName,
		Platform:      platformName,
		SessionKey:    msg.SessionKey,
		Filter:        filter,
		ExcludeFilter: excludeFilter,
		Prompt:        prompt,
		Interval:      interval,
		Enabled:       true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := e.subscriptionManager.AddSubscription(sub); err != nil {
		if errors.Is(err, ErrSubscriptionDuplicate) {
			e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAlreadyExists))
			return
		}
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgError, err))
		return
	}

	humanInterval := CronExprToHuman(sub.Interval, e.i18n.CurrentLang())
	e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubCreated), sub.ID, sub.Filter, sub.ExcludeFilter, sub.Prompt, humanInterval))
}

func (e *Engine) cmdSubscribeList(p Platform, msg *Message, args []string) {
	var subs []*Subscription
	if len(args) > 0 && args[0] == "all" {
		subs = e.subscriptionManager.Store().ListByProject(e.name)
	} else {
		chatID := extractChannelID(msg.SessionKey)
		subs = e.subscriptionManager.Store().ListByChatID(chatID)
	}

	if len(subs) == 0 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgCronEmpty))
		return
	}

	lang := e.i18n.CurrentLang()
	var sb strings.Builder
	if len(args) > 0 && args[0] == "all" {
		sb.WriteString(fmt.Sprintf(e.i18n.T(MsgSubListAllTitle), len(subs)))
	} else {
		sb.WriteString(fmt.Sprintf(e.i18n.T(MsgSubListTitle), len(subs)))
	}
	sb.WriteString("\n\n")

	for i, s := range subs {
		if i > 0 {
			sb.WriteString("\n")
		}
		status := "✅"
		if !s.Enabled {
			status = "⏸"
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", status, s.Filter))
		sb.WriteString(fmt.Sprintf("ID: %s\n", s.ID))
		if s.ExcludeFilter != "" {
			sb.WriteString(fmt.Sprintf("Exclude: %s\n", s.ExcludeFilter))
		}
		human := CronExprToHuman(s.Interval, lang)
		sb.WriteString(e.i18n.Tf(MsgCronScheduleLabel, human, s.Interval))
	}

	e.reply(p, msg.ReplyCtx, sb.String())
}

func (e *Engine) cmdSubscribeShow(p Platform, msg *Message, args []string) {
	if len(args) == 0 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
		return
	}

	s := e.subscriptionManager.Store().Get(args[0])
	if s == nil {
		e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubNotFound), args[0]))
		return
	}

	lastRun := "-"
	if !s.LastRun.IsZero() {
		lastRun = s.LastRun.Format(time.RFC3339)
	}
	lastError := "-"
	if s.LastError != "" {
		lastError = s.LastError
	}
	anchor := "-"
	if s.Anchor != "" {
		anchor = s.Anchor
	}

	e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubShowFormat),
		s.ID, s.ChatID, s.Filter, s.ExcludeFilter, s.Prompt,
		CronExprToHuman(s.Interval, e.i18n.CurrentLang()),
		s.Enabled, s.ConsecutiveErrors, lastRun, lastError, anchor,
	))
}

func (e *Engine) cmdSubscribeEnable(p Platform, msg *Message, args []string) {
	if len(args) == 0 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
		return
	}
	if !e.isAdmin(msg.UserID) {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAdminRequired))
		return
	}
	if err := e.subscriptionManager.EnableSubscription(args[0]); err != nil {
		e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubNotFound), args[0]))
		return
	}
	e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubEnabled), args[0]))
}

func (e *Engine) cmdSubscribeDisable(p Platform, msg *Message, args []string) {
	if len(args) == 0 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
		return
	}
	if !e.isAdmin(msg.UserID) {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAdminRequired))
		return
	}
	if err := e.subscriptionManager.DisableSubscription(args[0]); err != nil {
		e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubNotFound), args[0]))
		return
	}
	e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubDisabled), args[0]))
}

func (e *Engine) cmdSubscribeDel(p Platform, msg *Message, args []string) {
	if len(args) == 0 {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubHelp))
		return
	}
	if !e.isAdmin(msg.UserID) {
		e.reply(p, msg.ReplyCtx, e.i18n.T(MsgSubAdminRequired))
		return
	}
	if err := e.subscriptionManager.RemoveSubscription(args[0]); err != nil {
		e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubNotFound), args[0]))
		return
	}
	e.reply(p, msg.ReplyCtx, fmt.Sprintf(e.i18n.T(MsgSubDeleted), args[0]))
}
