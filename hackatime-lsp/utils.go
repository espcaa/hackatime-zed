package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func buildHeartbeatArgs(hb Heartbeat) []string {
	args := []string{}

	args = append(args, "--entity", quoteArg(hb.Entity))
	args = append(args, "--time", fmt.Sprintf("%.3f", hb.Time))
	args = append(args, "--plugin", quoteArg(hb.Plugin))
	args = append(args, "--lineno", strconv.Itoa(hb.LineNumber))
	args = append(args, "--cursorpos", strconv.Itoa(hb.CursorPos))
	args = append(args, "--lines-in-file", strconv.Itoa(hb.Lines))

	if hb.Category != "" {
		args = append(args, "--category", hb.Category)
	}

	if hb.AILineChanges > 0 {
		args = append(args, "--ai-line-changes", strconv.Itoa(hb.AILineChanges))
	}
	if hb.HumanLineChanges > 0 {
		args = append(args, "--human-line-changes", strconv.Itoa(hb.HumanLineChanges))
	}

	if apiKey := getConfigValue("apiKey"); apiKey != "" {
		args = append(args, "--key", quoteArg(apiKey))
	}
	if apiUrl := getConfigValue("apiUrl"); apiUrl != "" {
		args = append(args, "--api-url", quoteArg(apiUrl))
	}

	if hb.AlternateProject != "" {
		args = append(args, "--alternate-project", quoteArg(hb.AlternateProject))
	}
	if hb.ProjectFolder != "" {
		args = append(args, "--project-folder", quoteArg(hb.ProjectFolder))
	}

	if hb.IsWrite {
		args = append(args, "--write")
	}

	if runtime.GOOS == "windows" {
		if configFile := getConfigFilePath(); configFile != "" {
			args = append(args, "--config", quoteArg(configFile))
		}
		if logFile := getLogFilePath(); logFile != "" {
			args = append(args, "--log-file", quoteArg(logFile))
		}
	}

	if hb.IsUnsaved {
		args = append(args, "--is-unsaved-entity")
	}

	if hb.LocalFile != "" {
		args = append(args, "--local-file", quoteArg(hb.LocalFile))
	}

	return args
}

func quoteArg(arg string) string {
	if needsQuoting(arg) {
		return "\"" + strings.ReplaceAll(arg, "\"", "\\\"") + "\""
	}
	return arg
}

func needsQuoting(arg string) bool {
	for _, ch := range arg {
		if ch == ' ' || ch == '\t' || ch == '"' || ch == '\\' {
			return true
		}
	}
	return false
}

func getConfigValue(key string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	configFile := filepath.Join(homeDir, ".wakatime.cfg")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

func getConfigFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".wakatime.cfg")
}

func getLogFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".wakatime", "wakatime.log")
}
