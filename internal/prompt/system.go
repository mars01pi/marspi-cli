package prompt

import (
	"os"
	"runtime"
	"strings"
)

type Options struct {
	ProjectRoot       string
	RulesPath         string
	SkillDescriptions string
	ToolNames         []string
}

type System struct {
	sections []string
}

func New(options Options) *System {
	tools := make(map[string]bool, len(options.ToolNames))
	for _, name := range options.ToolNames {
		tools[name] = true
	}
	sections := []string{
		"You are an interactive agent that helps users with software engineering tasks. " +
			"Use the instructions below and the tools available to you to assist the user.\n" +
			"Never invent URLs. Prefer absolute file paths.",
		"## Safety\n\nDestructive commands and any access outside the project root require explicit user confirmation.",
		"## Built-in Rules\n\n" +
			"1. Think before coding. State assumptions. If uncertain, ask rather than guess.\n" +
			"2. Minimum code. No features beyond what was asked.\n" +
			"3. Surgical changes. Touch only what you must. Match existing style.\n" +
			"4. Verify before completion. Transform tasks into verifiable goals.",
	}
	guidance := []string{"## Tool Selection"}
	if len(options.ToolNames) > 0 {
		guidance = append(guidance, "Available tools: "+strings.Join(options.ToolNames, ", ")+".",
			"Use a dedicated tool when one exists.")
	}
	if tools["bash"] {
		guidance = append(guidance, "Use **bash** when no dedicated tool fits.")
	}
	if tools["read"] && tools["bash"] {
		guidance = append(guidance, "Use bash to list directories; read only reads files.")
	}
	if tools["edit"] {
		guidance = append(guidance, "Use **edit** for small in-place changes; ensure `old` is unique or pass `all=true`.")
	}
	if tools["search_memory"] {
		guidance = append(guidance, "Use **search_memory** for your long-term notes, preferences, and past decisions.")
	}
	if tools["append_memory"] {
		guidance = append(guidance, "Use **append_memory** only for architecture decisions and persistent preferences.")
	}
	if tools["view_image"] {
		guidance = append(guidance, "Use **view_image** for screenshots, UI mockups, error screens, and diagrams.")
	}
	if tools["web_search"] {
		guidance = append(guidance, "Use **web_search** for information requiring the live web. Use at most 3 times per user query.")
	}
	if tools["attempt_completion"] {
		guidance = append(guidance, "Finish with **attempt_completion** to present the final result.")
	}
	sections = append(sections, strings.Join(guidance, "\n\n"))
	if tools["search_knowledge"] {
		sections = append(sections, "## Knowledge Base (RAG)\n\n"+
			"Call **search_knowledge** before making claims about internal policies, SOP, service rules, or case dossiers. "+
			"Cite returned markers like `[id=... | source=...]`. Do not invent clauses that were not retrieved.")
	}
	if tools["use_skill"] && options.SkillDescriptions != "" {
		sections = append(sections, "## Skills Selection Guidelines\n\n"+options.SkillDescriptions+
			"\n\nIf an installed skill is relevant, call use_skill before proceeding.")
	}
	rules, err := os.ReadFile(options.RulesPath)
	if err == nil && len(rules) > 0 {
		sections = append(sections, "## User Rules\n\n"+string(rules))
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "unknown"
	}
	sections = append(sections, "## Environment\n"+
		"- Working directory: "+options.ProjectRoot+"\n"+
		"- Operating system: "+runtime.GOOS+" ("+runtime.GOARCH+")\n"+
		"- Shell: "+shell)
	return &System{sections: sections}
}

func (system *System) Assemble() string {
	return strings.Join(system.sections, "\n\n")
}
