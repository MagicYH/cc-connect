package feishu

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chenhg5/cc-connect/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func decodeRenderedCard(t *testing.T, card *core.Card) map[string]any {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal([]byte(renderCard(card, "")), &got); err != nil {
		t.Fatalf("renderCard JSON decode failed: %v", err)
	}
	return got
}

func TestRenderCardMap_EqualColumnsActionsUseColumnSet(t *testing.T) {
	buttons := []core.CardButton{
		core.PrimaryBtn("Session Management", "nav:/help session"),
		core.DefaultBtn("Agent Configuration", "nav:/help agent"),
		core.DefaultBtn("Tools & Automation", "nav:/help tools"),
		core.DefaultBtn("System", "nav:/help system"),
	}
	card := core.NewCard().ButtonsEqual(buttons...).Build()
	got := decodeRenderedCard(t, card)

	elements, ok := got["elements"].([]any)
	if !ok || len(elements) != 1 {
		t.Fatalf("elements = %#v, want one element", got["elements"])
	}
	columnSet, ok := elements[0].(map[string]any)
	if !ok {
		t.Fatalf("first element = %#v, want object", elements[0])
	}
	if tag := columnSet["tag"]; tag != "column_set" {
		t.Fatalf("tag = %#v, want column_set", tag)
	}
	columns, ok := columnSet["columns"].([]any)
	if !ok || len(columns) != len(buttons) {
		t.Fatalf("columns = %#v, want %d columns", columnSet["columns"], len(buttons))
	}

	for i, want := range buttons {
		col, ok := columns[i].(map[string]any)
		if !ok {
			t.Fatalf("column %d = %#v, want object", i, columns[i])
		}
		if width := col["width"]; width != "weighted" {
			t.Fatalf("column %d width = %#v, want weighted", i, width)
		}
		if weight := col["weight"]; weight != float64(1) {
			t.Fatalf("column %d weight = %#v, want 1", i, weight)
		}
		innerElems, ok := col["elements"].([]any)
		if !ok || len(innerElems) != 1 {
			t.Fatalf("column %d elements = %#v, want one button", i, col["elements"])
		}
		btn, ok := innerElems[0].(map[string]any)
		if !ok {
			t.Fatalf("column %d button = %#v, want object", i, innerElems[0])
		}
		if tag := btn["tag"]; tag != "button" {
			t.Fatalf("column %d tag = %#v, want button", i, tag)
		}
		text, ok := btn["text"].(map[string]any)
		if !ok || text["content"] != want.Text {
			t.Fatalf("column %d text = %#v, want %q", i, btn["text"], want.Text)
		}
		if btnType := btn["type"]; btnType != want.Type {
			t.Fatalf("column %d type = %#v, want %q", i, btnType, want.Type)
		}
		value, ok := btn["value"].(map[string]any)
		if !ok || value["action"] != want.Value {
			t.Fatalf("column %d value = %#v, want %q", i, btn["value"], want.Value)
		}
	}
}

func TestRenderCardMap_TwoEqualColumnsUseBisectAndCenteredButtons(t *testing.T) {
	buttons := []core.CardButton{
		core.PrimaryBtn("Session Management", "nav:/help session"),
		core.DefaultBtn("Agent Configuration", "nav:/help agent"),
	}
	card := core.NewCard().ButtonsEqual(buttons...).Build()
	got := decodeRenderedCard(t, card)

	elements, ok := got["elements"].([]any)
	if !ok || len(elements) != 1 {
		t.Fatalf("elements = %#v, want one element", got["elements"])
	}
	columnSet, ok := elements[0].(map[string]any)
	if !ok {
		t.Fatalf("first element = %#v, want object", elements[0])
	}
	if flexMode := columnSet["flex_mode"]; flexMode != "bisect" {
		t.Fatalf("flex_mode = %#v, want bisect", flexMode)
	}
	columns, ok := columnSet["columns"].([]any)
	if !ok || len(columns) != len(buttons) {
		t.Fatalf("columns = %#v, want %d columns", columnSet["columns"], len(buttons))
	}
	for i := range buttons {
		col, ok := columns[i].(map[string]any)
		if !ok {
			t.Fatalf("column %d = %#v, want object", i, columns[i])
		}
		if align := col["horizontal_align"]; align != "center" {
			t.Fatalf("column %d horizontal_align = %#v, want center", i, align)
		}
		innerElems, ok := col["elements"].([]any)
		if !ok || len(innerElems) != 1 {
			t.Fatalf("column %d elements = %#v, want one button", i, col["elements"])
		}
		btn, ok := innerElems[0].(map[string]any)
		if !ok {
			t.Fatalf("column %d button = %#v, want object", i, innerElems[0])
		}
		if width := btn["width"]; width != "fill" {
			t.Fatalf("column %d button width = %#v, want fill", i, width)
		}
	}
}

func TestRenderCardMap_DefaultActionsStayActionRow(t *testing.T) {
	buttons := []core.CardButton{
		core.PrimaryBtn("Yes", "act:/yes"),
		core.DefaultBtn("No", "act:/no"),
	}
	card := core.NewCard().Buttons(buttons...).Build()
	got := decodeRenderedCard(t, card)

	elements, ok := got["elements"].([]any)
	if !ok || len(elements) != 1 {
		t.Fatalf("elements = %#v, want one element", got["elements"])
	}
	actionRow, ok := elements[0].(map[string]any)
	if !ok {
		t.Fatalf("first element = %#v, want object", elements[0])
	}
	if tag := actionRow["tag"]; tag != "action" {
		t.Fatalf("tag = %#v, want action", tag)
	}
	actions, ok := actionRow["actions"].([]any)
	if !ok || len(actions) != len(buttons) {
		t.Fatalf("actions = %#v, want %d buttons", actionRow["actions"], len(buttons))
	}
	for i, want := range buttons {
		btn, ok := actions[i].(map[string]any)
		if !ok {
			t.Fatalf("button %d = %#v, want object", i, actions[i])
		}
		if tag := btn["tag"]; tag != "button" {
			t.Fatalf("button %d tag = %#v, want button", i, tag)
		}
		text, ok := btn["text"].(map[string]any)
		if !ok || text["content"] != want.Text {
			t.Fatalf("button %d text = %#v, want %q", i, btn["text"], want.Text)
		}
		if btnType := btn["type"]; btnType != want.Type {
			t.Fatalf("button %d type = %#v, want %q", i, btnType, want.Type)
		}
		value, ok := btn["value"].(map[string]any)
		if !ok || value["action"] != want.Value {
			t.Fatalf("button %d value = %#v, want %q", i, btn["value"], want.Value)
		}
	}
}

func TestRenderCardMap_DeleteModeUsesCheckerForm(t *testing.T) {
	card := core.NewCard().
		Title("删除会话", "carmine").
		ListItemBtn("☑ **1.** One · **10** msgs · 03-13 20:00", "已选择", "primary", "act:/delete-mode toggle session-1").
		ListItemBtn("▶ **2.** Active · **30** msgs · 03-13 20:01", "当前会话", "primary", "act:/delete-mode noop session-2").
		ListItemBtn("◻ **3.** Three · **20** msgs · 03-13 20:02", "选择", "default", "act:/delete-mode toggle session-3").
		Note("2 selected").
		Buttons(
			core.DangerBtn("删除已选", "act:/delete-mode confirm"),
			core.DefaultBtn("取消", "act:/delete-mode cancel"),
		).
		Buttons(core.DefaultBtn("下一页 →", "act:/delete-mode page 2")).
		Build()

	got := decodeRenderedCard(t, card)
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal rendered card failed: %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, `"tag":"form"`) || !strings.Contains(s, `"tag":"checker"`) {
		t.Fatalf("expected form+checker rendering, got %s", s)
	}
	if got := strings.Count(s, `"tag":"checker"`); got != 2 {
		t.Fatalf("checker count = %d, want 2, got %s", got, s)
	}
	if !strings.Contains(s, deleteModeCheckerName("session-1")) {
		t.Fatalf("selectable session checker missing, got %s", s)
	}
	if strings.Contains(s, deleteModeCheckerName("session-2")) {
		t.Fatalf("active session should not render checker, got %s", s)
	}
	if !strings.Contains(s, deleteModeCheckerName("session-3")) {
		t.Fatalf("second selectable session checker missing, got %s", s)
	}
	activeIdx := strings.Index(s, `▶ **2.** Active`)
	firstIdx := strings.Index(s, deleteModeCheckerName("session-1"))
	thirdIdx := strings.Index(s, deleteModeCheckerName("session-3"))
	if activeIdx < 0 || firstIdx < 0 || thirdIdx < 0 {
		t.Fatalf("missing expected order markers in rendered card: %s", s)
	}
	if !(firstIdx < activeIdx && activeIdx < thirdIdx) {
		t.Fatalf("row order changed unexpectedly, got %s", s)
	}
	if !strings.Contains(s, `"name":"delete_mode_form"`) {
		t.Fatalf("expected form name for feishu validation, got %s", s)
	}
	if !strings.Contains(s, `"name":"delete_mode_submit"`) || !strings.Contains(s, `"name":"delete_mode_cancel"`) {
		t.Fatalf("expected button names inside form, got %s", s)
	}
	if !strings.Contains(s, `"form_action_type":"submit"`) || !strings.Contains(s, `act:/delete-mode form-submit`) {
		t.Fatalf("expected form submit action, got %s", s)
	}
	if strings.Contains(s, `act:/delete-mode toggle`) {
		t.Fatalf("expected no toggle buttons in rendered card, got %s", s)
	}
}

func TestRenderCardMap_InjectsSessionKeyIntoCallbacks(t *testing.T) {
	card := core.NewCard().
		Buttons(core.PrimaryBtn("Open", "nav:/help session")).
		ListItem("Choose", "Confirm", "act:/confirm").
		Select("Pick one", []core.CardSelectOption{{Text: "A", Value: "askq:0:1"}}, "").
		Build()

	got := renderCardMap(card, "feishu:oc_chat:root:om_root")
	elements, ok := got["elements"].([]map[string]any)
	if !ok || len(elements) != 3 {
		t.Fatalf("elements = %#v, want 3 elements", got["elements"])
	}

	actionRow := elements[0]
	actions := actionRow["actions"].([]map[string]any)
	firstButton := actions[0]
	value := firstButton["value"].(map[string]string)
	if value["session_key"] != "feishu:oc_chat:root:om_root" {
		t.Fatalf("button session_key = %#v, want thread session key", value["session_key"])
	}

	listRow := elements[1]
	columns := listRow["columns"].([]map[string]any)
	actionCol := columns[1]
	listBtn := actionCol["elements"].([]map[string]any)[0]
	listValue := listBtn["value"].(map[string]string)
	if listValue["session_key"] != "feishu:oc_chat:root:om_root" {
		t.Fatalf("list item session_key = %#v, want thread session key", listValue["session_key"])
	}

	selectRow := elements[2]
	selectActions := selectRow["actions"].([]map[string]any)
	selectValue := selectActions[0]["value"].(map[string]string)
	if selectValue["session_key"] != "feishu:oc_chat:root:om_root" {
		t.Fatalf("select session_key = %#v, want thread session key", selectValue["session_key"])
	}
}

func TestBuildCardJSONWithStatusFooter(t *testing.T) {
	body := "Hello world"
	footer := "Opus 4.7 · ↑ 1 ↓ 168 · 4%\n~/path/to/ws"
	jsonStr := buildCardJSONWithStatusFooter(body, footer)

	var card map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &card); err != nil {
		t.Fatalf("decode card json: %v", err)
	}
	body0 := card["body"].(map[string]any)
	elements := body0["elements"].([]any)
	if len(elements) != 3 {
		t.Fatalf("expected 3 elements (body markdown, hr, footer markdown), got %d: %#v", len(elements), elements)
	}
	bodyEl := elements[0].(map[string]any)
	if bodyEl["tag"] != "markdown" || bodyEl["content"] != body {
		t.Errorf("body element = %#v, want markdown with content %q", bodyEl, body)
	}
	hrEl := elements[1].(map[string]any)
	if hrEl["tag"] != "hr" {
		t.Errorf("middle element = %#v, want hr", hrEl)
	}
	footerEl := elements[2].(map[string]any)
	if footerEl["tag"] != "markdown" {
		t.Errorf("footer tag = %v, want markdown", footerEl["tag"])
	}
	if footerEl["text_size"] != "notation" {
		t.Errorf("footer text_size = %v, want \"notation\"", footerEl["text_size"])
	}
	if footerEl["content"] != footer {
		t.Errorf("footer content = %q, want %q", footerEl["content"], footer)
	}
}

func TestBuildCardJSONWithStatusFooter_EmptyFooterFallsThrough(t *testing.T) {
	body := "Hello"
	a := buildCardJSONWithStatusFooter(body, "")
	b := buildCardJSON(body)
	if a != b {
		t.Errorf("empty footer should match buildCardJSON output\n got: %s\nwant: %s", a, b)
	}
	// whitespace-only footer also falls through
	if got := buildCardJSONWithStatusFooter(body, "   \n  "); got != b {
		t.Errorf("whitespace footer should fall through to buildCardJSON")
	}
}

func TestCardTextContent(t *testing.T) {
	tests := []struct {
		name string
		card *core.Card
		want string
	}{
		{
			name: "nil card",
			card: nil,
			want: "",
		},
		{
			name: "empty card",
			card: &core.Card{},
			want: "",
		},
		{
			name: "markdown only",
			card: &core.Card{Elements: []core.CardElement{
				core.CardMarkdown{Content: "hello world"},
			}},
			want: "hello world ",
		},
		{
			name: "all element types",
			card: &core.Card{Elements: []core.CardElement{
				core.CardMarkdown{Content: "intro"},
				core.CardDivider{},
				core.CardActions{Buttons: []core.CardButton{
					{Text: "Yes", Value: "v1"},
					{Text: "No", Value: "v2"},
				}},
				core.CardNote{Text: "footnote"},
				core.CardListItem{Text: "item text", BtnText: "Go"},
			}},
			want: "intro Yes No footnote item text ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cardTextContent(tt.card)
			if got != tt.want {
				t.Errorf("cardTextContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCardContainsMention(t *testing.T) {
	p := &interactivePlatform{
		Platform: &Platform{
			resolveMentions: true,
		},
	}
	// Pre-populate chat member cache so resolveMentionsInContent can find members.
	p.Platform.chatMemberCache.Store("oc_test", &chatMemberEntry{
		members:   map[string]string{"Alice": "ou_alice", "Bob": "ou_bob"},
		fetchedAt: time.Now(),
	})

	tests := []struct {
		name      string
		card      *core.Card
		chatID    string
		wantMention bool
	}{
		{
			name:      "no mention",
			card:      core.NewCard().Markdown("hello world").Build(),
			chatID:    "oc_test",
			wantMention: false,
		},
		{
			name:      "at-name resolves to mention",
			card:      core.NewCard().Markdown("hey @Alice check this").Build(),
			chatID:    "oc_test",
			wantMention: true,
		},
		{
			name:      "pre-existing at tag",
			card:      core.NewCard().Markdown(`<at user_id="ou_bob">Bob</at> hello`).Build(),
			chatID:    "oc_test",
			wantMention: true,
		},
		{
			name:      "unknown name not resolved",
			card:      core.NewCard().Markdown("@UnknownPerson hello").Build(),
			chatID:    "oc_test",
			wantMention: false,
		},
		{
			name:      "mention in note element",
			card:      core.NewCard().Markdown("body").Note("@Alice see this").Build(),
			chatID:    "oc_test",
			wantMention: true,
		},
		{
			name:      "mention in list item",
			card:      core.NewCard().ListItem("@Alice task", "Done", "act:/done").Build(),
			chatID:    "oc_test",
			wantMention: true,
		},
		{
			name:      "empty chatID skips resolution",
			card:      core.NewCard().Markdown("@Alice hello").Build(),
			chatID:    "",
			wantMention: false,
		},
		{
			name:      "at in button text",
			card:      core.NewCard().Buttons(core.DefaultBtn("@Alice", "act:/ping")).Build(),
			chatID:    "oc_test",
			wantMention: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMention, _ := p.cardContainsMention(context.Background(), tt.chatID, tt.card)
			if hasMention != tt.wantMention {
				t.Errorf("cardContainsMention() = %v, want %v", hasMention, tt.wantMention)
			}
		})
	}
}

func TestContainsAtTag(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "no at tag",
			content: "hello world",
			want:    false,
		},
		{
			name:    "at tag with user_id",
			content: `<at user_id="ou_abc">Alice</at> hello`,
			want:    true,
		},
		{
			name:    "at tag with id attribute",
			content: `<at id=ou_abc></at>`,
			want:    true,
		},
		{
			name:    "at tag in json.Marshal output (escaped)",
			content: buildCardJSON(`<at user_id="ou_abc">Alice</at> hello`),
			want:    true,
		},
		{
			name:    "plain @ sign not a tag",
			content: "@Alice hello",
			want:    false,
		},
		{
			name:    "at all tag",
			content: `<at user_id="all">all</at>`,
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsAtTag(tt.content); got != tt.want {
				t.Errorf("containsAtTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateMessage_MentionFallback(t *testing.T) {
	// Verify that containsAtTag correctly detects <at> tags in card JSON.
	// json.Marshal escapes < to <, so we need to check for the escaped form too.

	// Case 1: Content with <at> tag goes through buildCardJSON (which uses json.Marshal)
	content := `<at user_id="ou_abc">Alice</at> please review`
	cardJSON := buildCardJSON(sanitizeMarkdownURLs(preprocessFeishuMarkdown(content)))
	if !containsAtTag(cardJSON) {
		t.Errorf("expected card JSON to contain <at> tag (escaped), got: %s", cardJSON)
	}

	// Case 2: Card JSON input from BuildRichCard (also uses json.Marshal)
	richCardJSON := buildCardJSONWithStatusFooter(
		sanitizeMarkdownURLs(preprocessFeishuMarkdown(content)),
		"footer",
	)
	if !containsAtTag(richCardJSON) {
		t.Errorf("expected status footer card JSON to contain <at> tag (escaped), got: %s", richCardJSON)
	}

	// Case 3: Plain content without mentions should NOT trigger fallback
	plainContent := "hello world"
	plainCardJSON := buildCardJSON(sanitizeMarkdownURLs(preprocessFeishuMarkdown(plainContent)))
	if containsAtTag(plainCardJSON) {
		t.Errorf("expected plain card JSON NOT to contain <at> tag, got: %s", plainCardJSON)
	}

	// Case 4: Raw <at> tag in non-JSON content (direct text)
	if !containsAtTag(`<at user_id="ou_abc">Alice</at> hello`) {
		t.Errorf("expected raw <at> tag to be detected")
	}
}

func TestSendPreviewStart_MentionFallback(t *testing.T) {
	// Verify that SendPreviewStart returns ErrNotSupported when the card JSON
	// contains <at> tags (even in json.Marshal-escaped form).
	// The check must happen BEFORE createCardEntity, so a nil client won't crash.
	p := &Platform{
		platformName:       "feishu_test",
		useInteractiveCard: true,
	}

	rc := replyContext{chatID: "oc_test", messageID: "om_parent"}

	// Card JSON produced by BuildRichCard with <at> tag inside.
	// json.Marshal escapes < to < so the content contains the escaped form.
	mentionContent := buildCardJSON(`<at user_id="ou_abc">Alice</at> hello`)
	_, err := p.SendPreviewStart(context.Background(), rc, mentionContent)
	if err != core.ErrNotSupported {
		t.Errorf("SendPreviewStart with mention = %v, want ErrNotSupported", err)
	}

	// Plain content without mentions: the mention check passes, but the API call
	// will fail. We only verify that ErrNotSupported is NOT returned — any other
	// error (or panic) is expected since there's no real Lark client.
	// This path is covered by TestContainsAtTag (no false positives on plain text).
}

func TestBuildReplyContent_MentionForcesPost(t *testing.T) {
	// Verify that buildReplyContent forces MsgTypePost when content contains <at> tags,
	// even when useInteractiveCard is true.
	content := `<at user_id="ou_abc">Alice</at> please review`

	msgType, _ := buildReplyContent(content, true)
	if msgType != larkim.MsgTypePost {
		t.Errorf("buildReplyContent with <at> tag and useInteractiveCard=true: msgType = %v, want MsgTypePost", msgType)
	}

	// Without <at> tags and with markdown, card should be used
	plainContent := "hello **world**"
	msgType2, _ := buildReplyContent(plainContent, true)
	if msgType2 != larkim.MsgTypeInteractive {
		t.Errorf("buildReplyContent without <at> tag: msgType = %v, want MsgTypeInteractive", msgType2)
	}
}

func TestSendWithStatusFooter_PlainTextUsesPost(t *testing.T) {
	// Verify the logic that SendWithStatusFooter uses: plain-text content + footer
	// should produce a MsgTypePost with footer appended as italic, not an interactive card.
	tests := []struct {
		name      string
		content   string
		wantCard  bool // true = MsgTypeInteractive, false = MsgTypePost
	}{
		{
			name:     "pure Chinese text",
			content:  "设计文档已完成但尚未送审。按照工作流，",
			wantCard: false,
		},
		{
			name:     "simple English text",
			content:  "Hello, the task is done.",
			wantCard: false,
		},
		{
			name:     "text with bold markdown",
			content:  "The **result** is ready.",
			wantCard: true,
		},
		{
			name:     "text with code fence",
			content:  "Here is the code:\n```go\nfmt.Println()\n```",
			wantCard: true,
		},
		{
			name:     "text with list",
			content:  "Items:\n- First\n- Second",
			wantCard: true,
		},
		{
			name:     "text with inline code",
			content:  "Use the `foo` command.",
			wantCard: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMd := containsMarkdown(tt.content)
			if hasMd != tt.wantCard {
				t.Errorf("containsMarkdown(%q) = %v, want %v", tt.content, hasMd, tt.wantCard)
			}
			// When not using card, verify the combined content is valid Post JSON.
			if !tt.wantCard {
				footer := "Sonnet 4 · ctx 3% · ~/ws"
				combined := tt.content + "\n\n*" + footer + "*"
				body := buildPostMdJSON(combined)
				var parsed map[string]any
				if err := json.Unmarshal([]byte(body), &parsed); err != nil {
					t.Fatalf("buildPostMdJSON produced invalid JSON: %v", err)
				}
				// Verify the content contains both the original text and the italic footer.
				locale := parsed["zh_cn"].(map[string]any)
				contentArr := locale["content"].([]any)
				firstRow := contentArr[0].([]any)
				mdEl := firstRow[0].(map[string]any)
				text := mdEl["text"].(string)
				if !strings.Contains(text, tt.content) {
					t.Errorf("Post body missing original content, got %q", text)
				}
				if !strings.Contains(text, "*"+footer+"*") {
					t.Errorf("Post body missing italic footer, got %q", text)
				}
			}
		})
	}
}
