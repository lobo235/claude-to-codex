package main

import (
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
)

const inheritEnvVar = "CLAUDE_BRIDGE_INHERIT_ENV"

var baselineEnvKeys = []string{
	"PATH",
	"HOME",
	"USER",
	"LOGNAME",
	"SHELL",
	"TMPDIR",
	"TEMP",
	"TMP",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"SSL_CERT_FILE",
	"SSL_CERT_DIR",
}

func buildChildEnv(server ScopedServer) []string {
	if inheritAllEnv(server.Config) {
		return mergeEnv(os.Environ(), explicitServerEnv(server))
	}
	return mergeEnv(baselineEnv(), explicitServerEnv(server))
}

func inheritAllEnv(cfg MCPServerConfig) bool {
	return cfg.InheritEnv || os.Getenv(inheritEnvVar) == "1"
}

func baselineEnv() []string {
	env := make([]string, 0, len(baselineEnvKeys))
	for _, key := range baselineEnvKeys {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func explicitServerEnv(server ScopedServer) []string {
	keys := sortedKeys(server.Config.Env)
	env := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		env = append(env, key+"="+server.Config.Env[key])
	}
	if server.Scope == "project" && server.WorkDir != "" {
		env = append(env, "CLAUDE_BRIDGE_PROJECT_ROOT="+server.WorkDir)
	}
	return env
}

func mergeEnv(base, overrides []string) []string {
	values := map[string]string{}
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key != "" {
			values[key] = value
		}
	}
	for _, entry := range overrides {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key != "" {
			values[key] = value
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

var urlLikePattern = regexp.MustCompile(`https?://[^\s"'<>]+`)
var assignmentSecretPattern = regexp.MustCompile(`(?i)\b([a-z0-9_.-]*(?:token|key|secret|password|passwd|auth|credential|session|sig|signature)[a-z0-9_.-]*)(\s*[=:]\s*)([^\s,;&]+)`)
var bearerPattern = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/\-=]+`)
var rawTokenPattern = regexp.MustCompile(`\b(?:github_pat_[A-Za-z0-9_]{20,}|gh[pousr]_[A-Za-z0-9_]{20,}|sk-(?:proj-)?[A-Za-z0-9_-]{20,}|xox[baprs]-[A-Za-z0-9-]{10,}|[a-z]{4}_[A-Za-z0-9_-]{20,})\b`)
var jwtPattern = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)

func redactSensitive(text string) string {
	text = urlLikePattern.ReplaceAllStringFunc(text, redactURL)
	text = bearerPattern.ReplaceAllString(text, "Bearer [REDACTED]")
	text = assignmentSecretPattern.ReplaceAllString(text, "$1$2[REDACTED]")
	text = rawTokenPattern.ReplaceAllString(text, "[REDACTED]")
	text = jwtPattern.ReplaceAllString(text, "[REDACTED]")
	return text
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return redactSensitiveQuery(raw)
	}
	if u.User != nil {
		u.User = url.User("[REDACTED]")
	}
	query := u.Query()
	for key := range query {
		if sensitiveName(key) {
			query.Set(key, "[REDACTED]")
		}
	}
	u.RawQuery = query.Encode()
	u.Path = redactSensitivePath(u.Path)
	return u.String()
}

func redactSensitivePath(path string) string {
	segments := strings.Split(path, "/")
	for i := 1; i < len(segments); i++ {
		previous := segments[i-1]
		if sensitiveName(previous) {
			segments[i] = "[REDACTED]"
			continue
		}
		segments[i] = redactSensitiveSegment(segments[i])
	}
	return strings.Join(segments, "/")
}

func redactSensitiveSegment(segment string) string {
	if strings.Contains(segment, "=") {
		return redactSensitive(segment)
	}
	return segment
}

func redactSensitiveQuery(raw string) string {
	parts := strings.Split(raw, "&")
	for i, part := range parts {
		key, _, ok := strings.Cut(part, "=")
		if ok && sensitiveName(key) {
			parts[i] = key + "=[REDACTED]"
		}
	}
	return strings.Join(parts, "&")
}

func sensitiveName(name string) bool {
	name = strings.ToLower(name)
	for _, term := range []string{"token", "key", "secret", "password", "passwd", "auth", "credential", "session", "sig", "signature"} {
		if strings.Contains(name, term) {
			return true
		}
	}
	return false
}
