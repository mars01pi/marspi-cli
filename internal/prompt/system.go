// Package prompt 分层装配系统提示词，对齐 mangopi 的 SystemPrompt。
package prompt

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/skill"
)

// System 负责按 section 组装完整系统提示词。
type System struct {
	cfg      *config.Config
	skills   *skill.Manager
	sections []section
}

type section struct {
	name    string
	content string
}

// NewSystem 构建 System 并加载默认 sections。
func NewSystem(cfg *config.Config, skills *skill.Manager) *System {
	s := &System{cfg: cfg, skills: skills}
	s.sections = []section{
		{"base_intro", baseIntro()},
		{"safety", safetySection()},
		{"builtin_rules", builtinRules()},
		{"tool_guidance", toolGuidance()},
		{"skills_guidance", s.skillsGuidance()},
		{"memory", s.userRules()},
		{"environment", s.environment()},
	}
	return s
}

// Assemble 拼接全部 section。
func (s *System) Assemble() string {
	parts := make([]string, 0, len(s.sections))
	for _, sec := range s.sections {
		parts = append(parts, sec.content)
	}
	return strings.Join(parts, "\n\n")
}

func baseIntro() string {
	return "You are an interactive agent that helps users with software engineering tasks. " +
		"Use the instructions below and the tools available to you to assist the user.\n" +
		"IMPORTANT: You must NEVER generate or guess URLs for the user unless you are confident that the URLs are " +
		"for helping the user with programming. For file paths, always prefer absolute paths when possible. If " +
		"you need to read a directory, use the bash tool (ls) because the read tool cannot read directories.\n"
}

func toolGuidance() string {
	return "## Tool Selection\n\n" +
		"Use the dedicated tool when one exists (read/write/edit/search/grep/search_memory/" +
		"append_memory/attempt_completion). Reach for **bash** only when no dedicated tool fits.\n" +
		"Use **edit** (not write) for small in-place changes; ensure `old` is unique or pass `all=true`.\n" +
		"Use **search_memory** for long-term knowledge, **append_memory** only for " +
		"architecture decisions / persistent preferences (not ephemeral context).\n" +
		"Use **view_image** for screenshots, UI mockups, error screens, and diagrams. " +
		"The `read` tool auto-routes image files (.png/.jpg/.jpeg/.gif/.webp) to vision, " +
		"but call `view_image` directly when the path is computed or generated.\n" +
		"Use **web_search** for the latest docs, news, or anything that requires the live web " +
		"beyond the local filesystem. Requires the `MARS_SEARCH_API_KEY` env var. " +
		"Use sparingly — at most 3 times per user query to avoid excessive API calls.\n" +
		"Always finish with **attempt_completion** to present the final result.\n\n"
}

func (s *System) skillsGuidance() string {
	desc := s.skills.Descriptions()
	if desc == "" {
		return "## Skills Selection Guidelines\n\nNo skills available.\n\n"
	}
	return "## Skills Selection Guidelines\n\n" + desc + "\n\n" +
		"- If an installed skill is relevant, call use_skill first before proceeding.\n" +
		"- Skills may contain: workflows, best practices, reusable scripts, references\n\n"
}

func (s *System) environment() string {
	osInfo := runtime.GOOS + " (" + runtime.GOARCH + ")"
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "unknown"
	}
	return "## Environment\n" +
		"- Working directory: " + s.cfg.ProjectRoot + "\n" +
		"- Operating system: " + osInfo + "\n" +
		"- Shell: " + shell + "\n"
}

func (s *System) userRules() string {
	path := filepath.Join(s.cfg.ProjectRoot, ".mangocli", "MARS.md")
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return "## User Rules\n\nNo user-defined rules.\n"
	}
	return "## User Rules\n\n" + string(data)
}

func safetySection() string {
	return "## Safety\n\n" +
		"Destructive commands and any access outside the project root require explicit user confirmation.\n\n"
}

func builtinRules() string {
	return "## Built-in Rules\n\n" +
		"**1. Think before coding.** State assumptions. If uncertain, ask rather than guess.\n" +
		"**2. Minimum code.** If 200 lines can be 50, rewrite. No features beyond what was asked.\n" +
		"**3. Surgical changes.** Touch only what you must. Don't 'improve' adjacent code or " +
		"refactor things that aren't broken. Match existing style.\n" +
		"**4. Verify before completion.** Transform tasks into verifiable goals: " +
		"'Write tests for X, then make them pass.' For multi-step work, state a brief plan first.\n\n"
}
