package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const generatedSkillMarker = "generated-by: claude-to-codex sync-commands"

type commandSyncResult struct {
	Name        string
	SourcePath  string
	SkillPath   string
	Status      string
	Description string
	Reason      string
}

type skillSyncResult struct {
	Name      string
	SourceDir string
	SkillDir  string
	Status    string
	Reason    string
}

type claudeCommand struct {
	Name         string
	SourcePath   string
	Description  string
	ArgumentHint string
}

func runSyncCommands(args []string) error {
	fs := flag.NewFlagSet("sync-commands", flag.ContinueOnError)
	quiet := fs.Bool("quiet", false, "suppress unchanged sync output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected sync-commands arguments: %s", strings.Join(fs.Args(), " "))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	results, err := syncClaudeCommands(home)
	if err != nil {
		return err
	}
	if *quiet {
		return nil
	}
	if len(results) == 0 {
		fmt.Println("no Claude slash commands found")
		return nil
	}
	for _, result := range results {
		switch result.Status {
		case "created", "updated":
			fmt.Printf("%s: /%s -> %s\n", result.Status, result.Name, result.SkillPath)
		case "skipped":
			fmt.Printf("skipped: /%s (%s)\n", result.Name, result.Reason)
		}
	}
	return nil
}

func runSyncSkills(args []string) error {
	fs := flag.NewFlagSet("sync-skills", flag.ContinueOnError)
	quiet := fs.Bool("quiet", false, "suppress unchanged sync output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected sync-skills arguments: %s", strings.Join(fs.Args(), " "))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	results, err := syncClaudeSkills(home)
	if err != nil {
		return err
	}
	if *quiet {
		return nil
	}
	if len(results) == 0 {
		fmt.Println("no Claude skills found")
		return nil
	}
	for _, result := range results {
		switch result.Status {
		case "created":
			fmt.Printf("linked: %s -> %s\n", result.SkillDir, result.SourceDir)
		case "skipped":
			fmt.Printf("skipped: %s (%s)\n", result.Name, result.Reason)
		}
	}
	return nil
}

func syncClaudeSkills(home string) ([]skillSyncResult, error) {
	skills, err := loadClaudeSkills(filepath.Join(home, ".claude", "skills"))
	if err != nil {
		return nil, err
	}
	codexSkillsRoot := filepath.Join(home, ".codex", "skills")
	var results []skillSyncResult
	for _, skill := range skills {
		result, err := syncClaudeSkill(codexSkillsRoot, skill)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func loadClaudeSkills(skillsRoot string) ([]claudeSkill, error) {
	entries, err := os.ReadDir(skillsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Claude skills dir: %w", err)
	}
	var skills []claudeSkill
	for _, entry := range entries {
		if !validSkillName(entry.Name()) {
			continue
		}
		sourceDir := filepath.Join(skillsRoot, entry.Name())
		if _, err := os.Stat(filepath.Join(sourceDir, "SKILL.md")); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("stat Claude skill %s: %w", sourceDir, err)
		}
		skills = append(skills, claudeSkill{Name: entry.Name(), SourceDir: sourceDir})
	}
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

type claudeSkill struct {
	Name      string
	SourceDir string
}

func syncClaudeSkill(codexSkillsRoot string, skill claudeSkill) (skillSyncResult, error) {
	dest := filepath.Join(codexSkillsRoot, skill.Name)
	result := skillSyncResult{Name: skill.Name, SourceDir: skill.SourceDir, SkillDir: dest}

	info, err := os.Lstat(dest)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(dest)
			if readErr != nil {
				return result, fmt.Errorf("read Codex skill symlink %s: %w", dest, readErr)
			}
			targetPath := target
			if !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(filepath.Dir(dest), targetPath)
			}
			same, sameErr := samePath(targetPath, skill.SourceDir)
			if sameErr != nil {
				return result, sameErr
			}
			if same {
				result.Status = "unchanged"
				return result, nil
			}
		}
		result.Status = "skipped"
		result.Reason = "Codex skill already exists and is not the matching Claude-skill symlink"
		return result, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("stat Codex skill %s: %w", dest, err)
	}

	if err := os.MkdirAll(codexSkillsRoot, 0o755); err != nil {
		return result, fmt.Errorf("create Codex skills dir: %w", err)
	}
	if err := os.Symlink(skill.SourceDir, dest); err != nil {
		return result, fmt.Errorf("link Codex skill %s: %w", dest, err)
	}
	result.Status = "created"
	return result, nil
}

func samePath(a, b string) (bool, error) {
	realA, err := filepath.EvalSymlinks(a)
	if err != nil {
		return false, fmt.Errorf("resolve path %s: %w", a, err)
	}
	realB, err := filepath.EvalSymlinks(b)
	if err != nil {
		return false, fmt.Errorf("resolve path %s: %w", b, err)
	}
	return realA == realB, nil
}

func syncClaudeCommands(home string) ([]commandSyncResult, error) {
	commands, err := loadClaudeCommands(filepath.Join(home, ".claude", "commands"))
	if err != nil {
		return nil, err
	}
	skillsRoot := filepath.Join(home, ".codex", "skills")
	var results []commandSyncResult
	for _, command := range commands {
		result, err := syncClaudeCommand(skillsRoot, command)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func loadClaudeCommands(commandsDir string) ([]claudeCommand, error) {
	entries, err := os.ReadDir(commandsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Claude commands dir: %w", err)
	}
	var commands []claudeCommand
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		if !validSkillName(name) {
			continue
		}
		path := filepath.Join(commandsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read Claude command %s: %w", path, err)
		}
		frontmatter := parseCommandFrontmatter(string(data))
		commands = append(commands, claudeCommand{
			Name:         name,
			SourcePath:   path,
			Description:  frontmatter["description"],
			ArgumentHint: frontmatter["argument-hint"],
		})
	}
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})
	return commands, nil
}

func syncClaudeCommand(skillsRoot string, command claudeCommand) (commandSyncResult, error) {
	skillPath := filepath.Join(skillsRoot, command.Name, "SKILL.md")
	result := commandSyncResult{
		Name:        command.Name,
		SourcePath:  command.SourcePath,
		SkillPath:   skillPath,
		Description: command.Description,
	}

	existing, err := os.ReadFile(skillPath)
	if err == nil && !strings.Contains(string(existing), generatedSkillMarker) {
		result.Status = "skipped"
		result.Reason = "Codex skill already exists and is not bridge-generated"
		return result, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("read existing Codex skill %s: %w", skillPath, err)
	}

	body := renderCodexSkill(command)
	if err == nil && string(existing) == body {
		result.Status = "unchanged"
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		return result, fmt.Errorf("create Codex skill dir: %w", err)
	}
	if err := os.WriteFile(skillPath, []byte(body), 0o644); err != nil {
		return result, fmt.Errorf("write Codex skill %s: %w", skillPath, err)
	}
	if errors.Is(err, os.ErrNotExist) {
		result.Status = "created"
	} else {
		result.Status = "updated"
	}
	return result, nil
}

func parseCommandFrontmatter(body string) map[string]string {
	out := map[string]string{}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if !strings.HasPrefix(body, "---\n") {
		return out
	}
	end := strings.Index(body[4:], "\n---")
	if end < 0 {
		return out
	}
	lines := strings.Split(body[4:4+end], "\n")
	for i := 0; i < len(lines); i++ {
		key, value, ok := strings.Cut(lines[i], ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "description", "argument-hint":
			if value == ">-" || value == ">" || value == "|-" || value == "|" {
				var parts []string
				for i+1 < len(lines) && strings.HasPrefix(lines[i+1], "  ") {
					i++
					parts = append(parts, strings.TrimSpace(lines[i]))
				}
				out[key] = strings.Join(parts, " ")
			} else {
				out[key] = strings.Trim(value, `"'`)
			}
		}
	}
	return out
}

func renderCodexSkill(command claudeCommand) string {
	description := "Run Claude Code slash command /" + command.Name + " from " + command.SourcePath + "."
	if command.Description != "" {
		description += " Original command description: " + command.Description
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + command.Name + "\n")
	b.WriteString("description: >-\n")
	for _, line := range wrapText(description, 88) {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("---\n\n")
	b.WriteString("<!-- " + generatedSkillMarker + " -->\n\n")
	b.WriteString("# Bridged Claude Slash Command: /" + command.Name + "\n\n")
	b.WriteString("This skill is generated from the user-scoped Claude Code command at:\n\n")
	b.WriteString("- `" + command.SourcePath + "`\n\n")
	if command.ArgumentHint != "" {
		b.WriteString("Argument hint: `" + command.ArgumentHint + "`\n\n")
	}
	b.WriteString("## Workflow\n\n")
	b.WriteString("1. Read the source command file before doing work.\n")
	b.WriteString("2. Execute the source command's instructions in Codex, adapting Claude-specific tool names to Codex tools.\n")
	b.WriteString("3. Treat `allowed-tools` and `argument-hint` frontmatter as Claude Code metadata, not Codex permission changes.\n")
	b.WriteString("4. If the source command is missing or unreadable, say so and stop.\n")
	return b.String()
}

var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func validSkillName(name string) bool {
	return skillNamePattern.MatchString(name)
}

func wrapText(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	lines = append(lines, line)
	return lines
}
