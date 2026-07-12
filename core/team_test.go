package core

import (
	"strings"
	"testing"
)

func TestTeamRegistry_Compose_NoTeamReturnsBase(t *testing.T) {
	r := NewTeamRegistry()
	base := "You are helpful."
	if got := r.Compose("alpha", "", "desc", base); got != base {
		t.Fatalf("empty team should return base unchanged, got %q", got)
	}
}

func TestTeamRegistry_Compose_InjectsRoster(t *testing.T) {
	r := NewTeamRegistry()
	r.Add(TeamMember{Project: "leader", Team: "squad", MemberDescribe: "带队", OpenID: "ou_leader", AppName: "队长机器人"})
	r.Add(TeamMember{Project: "dev", Team: "squad", MemberDescribe: "写代码", OpenID: "ou_dev", AppName: "开发机器人"})
	r.Add(TeamMember{Project: "outsider", Team: "other", MemberDescribe: "无关", OpenID: "ou_x", AppName: "别的"})

	out := r.Compose("leader", "squad", "带队", "BASE PROMPT")

	if !strings.HasPrefix(out, "BASE PROMPT") {
		t.Fatalf("composed should start with base prompt, got: %q", out)
	}
	for _, want := range []string{
		TeamPromptMarker,
		"你的角色：leader（飞书应用：队长机器人）",
		"你的职责：带队",
		"dev（飞书应用：开发机器人）：写代码",
		`<at user_id="ou_dev"></at>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("composed missing %q\n---\n%s", want, out)
		}
	}
	// The member itself must not appear in the teammates roster.
	if strings.Contains(out, `<at user_id="ou_leader">`) {
		t.Errorf("self open_id should not be listed as a teammate:\n%s", out)
	}
	// A member of a different team must be excluded.
	if strings.Contains(out, "outsider") {
		t.Errorf("member of a different team leaked in:\n%s", out)
	}
}

func TestTeamRegistry_Compose_MissingOpenID(t *testing.T) {
	r := NewTeamRegistry()
	r.Add(TeamMember{Project: "a", Team: "t", MemberDescribe: "aa"})
	r.Add(TeamMember{Project: "b", Team: "t", MemberDescribe: "bb"}) // no open_id/app name

	out := r.Compose("a", "t", "aa", "")
	if !strings.Contains(out, "b：bb") {
		t.Errorf("teammate without app name should still be listed:\n%s", out)
	}
	if !strings.Contains(out, "open_id 未知") {
		t.Errorf("teammate without open_id should be flagged:\n%s", out)
	}
}

func TestTeamRegistry_Compose_NilRegistrySafe(t *testing.T) {
	var r *TeamRegistry
	if got := r.Compose("a", "", "d", "base"); got != "base" {
		t.Fatalf("nil registry empty team should return base, got %q", got)
	}
	out := r.Compose("a", "t", "d", "base")
	if !strings.Contains(out, TeamPromptMarker) {
		t.Fatalf("nil registry with team should still compose a block:\n%s", out)
	}
}
