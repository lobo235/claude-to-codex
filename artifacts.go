package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const artifactCacheEnv = "CLAUDE_TO_CODEX_ARTIFACT_CACHE_DIR"
const artifactCacheSchema = "schema-v1"
const artifactCacheLockWait = 2 * time.Minute
const artifactCacheStaleLock = 15 * time.Minute

type artifactSyncSummary struct {
	CacheRoot    string
	CodexHome    string
	Skills       []skillSyncResult
	Commands     []commandSyncResult
	Agents       []agentSyncResult
	Materialized int
}

func runSyncArtifacts(args []string) error {
	fs := flag.NewFlagSet("sync-artifacts", flag.ContinueOnError)
	quiet := fs.Bool("quiet", false, "suppress unchanged sync output")
	project := fs.String("project", "", "sync project-scoped agents from this project directory in addition to user artifacts")
	artifactCacheDir := fs.String("artifact-cache-dir", "", "persistent generated-artifact cache directory; defaults to "+artifactCacheEnv)
	codexHomeFlag := fs.String("codex-home", "", "allocation-local Codex home to materialize generated artifacts into; defaults to CODEX_HOME or ~/.codex")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected sync-artifacts arguments: %s", strings.Join(fs.Args(), " "))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cacheDir := firstNonEmpty(*artifactCacheDir, os.Getenv(artifactCacheEnv))
	if cacheDir == "" {
		return fmt.Errorf("--artifact-cache-dir or %s is required", artifactCacheEnv)
	}
	cacheDir, err = filepath.Abs(cacheDir)
	if err != nil {
		return fmt.Errorf("resolve artifact cache dir: %w", err)
	}
	codexHome := firstNonEmpty(*codexHomeFlag, os.Getenv("CODEX_HOME"), filepath.Join(home, ".codex"))
	codexHome, err = filepath.Abs(codexHome)
	if err != nil {
		return fmt.Errorf("resolve Codex home: %w", err)
	}

	var progress io.Writer
	if *quiet {
		progress = os.Stderr
	}
	summary, err := syncAndMaterializeArtifacts(home, *project, cacheDir, codexHome, progress)
	if err != nil {
		return err
	}
	if *quiet {
		return nil
	}
	fmt.Printf("synced automation artifacts: cache=%s codex_home=%s materialized=%d\n", summary.CacheRoot, summary.CodexHome, summary.Materialized)
	printArtifactResults(summary)
	return nil
}

func syncAndMaterializeArtifacts(home, project, cacheDir, codexHome string, progress io.Writer) (artifactSyncSummary, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return artifactSyncSummary{}, fmt.Errorf("create artifact cache dir: %w", err)
	}
	unlock, err := acquireArtifactCacheLock(cacheDir)
	if err != nil {
		return artifactSyncSummary{}, err
	}
	cacheRoot := automationCacheRoot(cacheDir)
	summary, syncErr := syncAutomationCache(home, project, cacheRoot, progress)
	unlockErr := unlock()
	if syncErr != nil {
		return artifactSyncSummary{}, syncErr
	}
	if unlockErr != nil {
		return artifactSyncSummary{}, unlockErr
	}
	materialized, err := materializeAutomationArtifacts(cacheRoot, project, codexHome)
	if err != nil {
		return artifactSyncSummary{}, err
	}
	summary.CacheRoot = cacheRoot
	summary.CodexHome = codexHome
	summary.Materialized = materialized
	return summary, nil
}

func syncAutomationCache(home, project, cacheRoot string, progress io.Writer) (artifactSyncSummary, error) {
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return artifactSyncSummary{}, fmt.Errorf("create versioned artifact cache dir: %w", err)
	}
	summary := artifactSyncSummary{CacheRoot: cacheRoot}

	skillsRoot := filepath.Join(cacheRoot, "skills")
	skills, err := syncClaudeSkillsTo(home, skillsRoot, progress)
	if err != nil {
		return artifactSyncSummary{}, err
	}
	summary.Skills = skills

	commands, err := syncClaudeCommandsTo(home, skillsRoot)
	if err != nil {
		return artifactSyncSummary{}, err
	}
	summary.Commands = commands

	scopes := []agentSyncScope{{
		Name:      "user",
		SourceDir: filepath.Join(home, ".claude", "agents"),
		TargetDir: filepath.Join(cacheRoot, "agents", "user"),
	}}
	if project != "" {
		scope, err := automationProjectAgentScope(cacheRoot, project)
		if err != nil {
			return artifactSyncSummary{}, err
		}
		scopes = append(scopes, scope)
	}
	agents, err := syncClaudeAgentsWithScopes(scopes, progress)
	if err != nil {
		return artifactSyncSummary{}, err
	}
	summary.Agents = agents
	return summary, nil
}

func materializeAutomationArtifacts(cacheRoot, project, codexHome string) (int, error) {
	count := 0
	skills, err := materializeSkillArtifacts(filepath.Join(cacheRoot, "skills"), filepath.Join(codexHome, "skills"))
	if err != nil {
		return 0, err
	}
	count += skills

	agentDirs := []string{filepath.Join(cacheRoot, "agents", "user")}
	if project != "" {
		projectDir, _, err := automationProjectAgentCacheDir(cacheRoot, project)
		if err != nil {
			return 0, err
		}
		agentDirs = append(agentDirs, projectDir)
	}
	for _, agentDir := range agentDirs {
		agents, err := materializeAgentArtifacts(agentDir, filepath.Join(codexHome, "agents"))
		if err != nil {
			return 0, err
		}
		count += agents
	}
	return count, nil
}

func materializeSkillArtifacts(cacheSkillsRoot, codexSkillsRoot string) (int, error) {
	entries, err := os.ReadDir(cacheSkillsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read cached skills dir: %w", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() || !validSkillName(entry.Name()) {
			continue
		}
		sourcePath := filepath.Join(cacheSkillsRoot, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(sourcePath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("read cached skill %s: %w", sourcePath, err)
		}
		if !containsAnyMarker(data, generatedClaudeSkillMarker, generatedCommandSkillMarker) {
			continue
		}
		destDir := filepath.Join(codexSkillsRoot, entry.Name())
		if symlink, err := isSymlink(destDir); err != nil {
			return 0, err
		} else if symlink {
			continue
		}
		wrote, err := writeGeneratedArtifact(filepath.Join(destDir, "SKILL.md"), data, generatedClaudeSkillMarker, generatedCommandSkillMarker)
		if err != nil {
			return 0, err
		}
		if wrote {
			count++
		}
	}
	return count, nil
}

func materializeAgentArtifacts(cacheAgentsRoot, codexAgentsRoot string) (int, error) {
	entries, err := os.ReadDir(cacheAgentsRoot)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read cached agents dir: %w", err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" || !validSkillName(strings.TrimSuffix(entry.Name(), ".toml")) {
			continue
		}
		sourcePath := filepath.Join(cacheAgentsRoot, entry.Name())
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return 0, fmt.Errorf("read cached agent %s: %w", sourcePath, err)
		}
		if !containsAnyMarker(data, generatedAgentMarker) {
			continue
		}
		wrote, err := writeGeneratedArtifact(filepath.Join(codexAgentsRoot, entry.Name()), data, generatedAgentMarker)
		if err != nil {
			return 0, err
		}
		if wrote {
			count++
		}
	}
	return count, nil
}

func writeGeneratedArtifact(path string, data []byte, markers ...string) (bool, error) {
	if !containsAnyMarker(data, markers...) {
		return false, nil
	}
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refuse to overwrite symlinked generated artifact %s", path)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("stat generated artifact %s: %w", path, err)
	}
	existing, readErr := os.ReadFile(path)
	if readErr == nil {
		if !containsAnyMarker(existing, markers...) {
			return false, nil
		}
		if bytes.Equal(existing, data) {
			return false, nil
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return false, fmt.Errorf("read generated artifact %s: %w", path, readErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create generated artifact dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("write generated artifact %s: %w", path, err)
	}
	return true, nil
}

func containsAnyMarker(data []byte, markers ...string) bool {
	body := string(data)
	for _, marker := range markers {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

func automationProjectAgentScope(cacheRoot, project string) (agentSyncScope, error) {
	targetDir, projectDir, err := automationProjectAgentCacheDir(cacheRoot, project)
	if err != nil {
		return agentSyncScope{}, err
	}
	return agentSyncScope{
		Name:       "project",
		SourceDir:  filepath.Join(projectDir, ".claude", "agents"),
		TargetDir:  targetDir,
		ProjectDir: projectDir,
	}, nil
}

func automationProjectAgentCacheDir(cacheRoot, project string) (string, string, error) {
	projectDir, err := filepath.Abs(project)
	if err != nil {
		return "", "", fmt.Errorf("resolve project dir: %w", err)
	}
	return filepath.Join(cacheRoot, "agents", "project-"+shortHash(projectDir)), projectDir, nil
}

func automationCacheRoot(cacheDir string) string {
	return filepath.Join(cacheDir, artifactCacheSchema, "claude-to-codex-"+safeCacheComponent(version))
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

var unsafeCacheComponentPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeCacheComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "dev"
	}
	value = unsafeCacheComponentPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, ".-_")
	if value == "" {
		return "dev"
	}
	return value
}

func acquireArtifactCacheLock(cacheDir string) (func() error, error) {
	lockDir := filepath.Join(cacheDir, ".sync.lock")
	deadline := time.Now().Add(artifactCacheLockWait)
	for {
		err := os.Mkdir(lockDir, 0o700)
		if err == nil {
			owner := fmt.Sprintf("pid=%d\nhost=%s\ntime=%s\n", os.Getpid(), hostnameForLock(), time.Now().Format(time.RFC3339Nano))
			if writeErr := os.WriteFile(filepath.Join(lockDir, "owner"), []byte(owner), 0o600); writeErr != nil {
				_ = os.RemoveAll(lockDir)
				return nil, fmt.Errorf("write artifact cache lock owner: %w", writeErr)
			}
			return func() error {
				if err := os.RemoveAll(lockDir); err != nil {
					return fmt.Errorf("release artifact cache lock: %w", err)
				}
				return nil
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire artifact cache lock: %w", err)
		}
		info, statErr := os.Stat(lockDir)
		if statErr == nil && time.Since(info.ModTime()) > artifactCacheStaleLock {
			_ = os.RemoveAll(lockDir)
			continue
		}
		if statErr != nil && errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("artifact cache lock is still held: %s", lockDir)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func hostnameForLock() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "unknown"
	}
	return hostname
}

func printArtifactResults(summary artifactSyncSummary) {
	for _, result := range summary.Skills {
		switch result.Status {
		case "created", "updated":
			fmt.Printf("%s skill: %s -> %s\n", result.Status, result.SourceDir, result.SkillDir)
		case "skipped":
			fmt.Printf("skipped skill: %s (%s)\n", result.Name, result.Reason)
		}
	}
	for _, result := range summary.Agents {
		switch result.Status {
		case "created", "updated", "deleted":
			fmt.Printf("%s agent: %s/%s -> %s\n", result.Status, result.Scope, result.Name, result.AgentPath)
		case "skipped":
			fmt.Printf("skipped agent: %s/%s (%s)\n", result.Scope, result.Name, result.Reason)
		}
	}
	for _, result := range summary.Commands {
		switch result.Status {
		case "created", "updated":
			fmt.Printf("%s command: /%s -> %s\n", result.Status, result.Name, result.SkillPath)
		case "skipped":
			fmt.Printf("skipped command: /%s (%s)\n", result.Name, result.Reason)
		}
	}
}
