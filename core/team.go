package core

import (
	"fmt"
	"sort"
	"strings"
)

// TeamMember captures one project's identity within a team. It is used to
// compose the team roster that cc-connect auto-injects into each member's
// system prompt.
type TeamMember struct {
	Project        string // project name, also used as the member's role name
	Team           string
	MemberDescribe string
	OpenID         string // Feishu bot open_id (empty when unresolved)
	AppName        string // Feishu bot app display name (empty when unresolved)
}

// TeamRegistry holds resolved team membership for all projects. It is built once
// at startup (after Feishu bot identities are fetched) and consulted both for
// startup system_prompt injection and the WebUI preview endpoint. A nil or empty
// registry composes to the base prompt unchanged.
type TeamRegistry struct {
	members map[string]TeamMember // project name -> member
}

// NewTeamRegistry returns an empty registry.
func NewTeamRegistry() *TeamRegistry {
	return &TeamRegistry{members: map[string]TeamMember{}}
}

// Add registers (or replaces) a member keyed by project name.
func (r *TeamRegistry) Add(m TeamMember) {
	if r.members == nil {
		r.members = map[string]TeamMember{}
	}
	r.members[m.Project] = m
}

// Member returns the stored member for a project (ok=false when absent).
func (r *TeamRegistry) Member(project string) (TeamMember, bool) {
	if r == nil {
		return TeamMember{}, false
	}
	m, ok := r.members[project]
	return m, ok
}

// TeamPromptMarker delimits the auto-injected block so it can be identified.
const TeamPromptMarker = "=== 团队协作(自动注入，请勿手动编辑)==="

// Compose appends a team roster block to base for the given member. team and
// memberDescribe describe the member itself (callers may pass live/edited
// values); teammates are drawn from the registry. When team is empty the base
// prompt is returned unchanged.
func (r *TeamRegistry) Compose(selfProject, team, memberDescribe, base string) string {
	team = strings.TrimSpace(team)
	if team == "" {
		return base
	}

	var self TeamMember
	var mates []TeamMember
	if r != nil {
		self = r.members[selfProject]
		for _, m := range r.members {
			if m.Project == selfProject {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(m.Team), team) {
				mates = append(mates, m)
			}
		}
	}
	sort.Slice(mates, func(i, j int) bool { return mates[i].Project < mates[j].Project })

	var b strings.Builder
	b.WriteString(strings.TrimRight(base, "\n"))
	b.WriteString("\n\n")
	b.WriteString(TeamPromptMarker)
	b.WriteString("\n")
	if self.AppName != "" {
		fmt.Fprintf(&b, "你是团队「%s」的成员。你的角色：%s（飞书应用：%s）。\n", team, selfProject, self.AppName)
	} else {
		fmt.Fprintf(&b, "你是团队「%s」的成员。你的角色：%s。\n", team, selfProject)
	}
	fmt.Fprintf(&b, "你的职责：%s\n", strings.TrimSpace(memberDescribe))

	if len(mates) == 0 {
		b.WriteString("\n团队暂无其他成员。\n")
		return b.String()
	}
	b.WriteString("\n团队其他成员：\n")
	for _, m := range mates {
		if m.AppName != "" {
			fmt.Fprintf(&b, "- %s（飞书应用：%s）：%s\n", m.Project, m.AppName, strings.TrimSpace(m.MemberDescribe))
		} else {
			fmt.Fprintf(&b, "- %s：%s\n", m.Project, strings.TrimSpace(m.MemberDescribe))
		}
		if strings.TrimSpace(m.OpenID) != "" {
			fmt.Fprintf(&b, "  @TA 时使用：<at user_id=\"%s\"></at>\n", m.OpenID)
		} else {
			b.WriteString("  （open_id 未知，暂无法 @）\n")
		}
	}
	return b.String()
}
