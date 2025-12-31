package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gantz-ai/gantz-cli/internal/config"
)

// Result represents script execution result
type Result struct {
	Output   string
	ExitCode int
	Duration time.Duration
	Error    error
}

// Executor runs scripts for tools
type Executor struct{}

// NewExecutor creates a new script executor
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute runs a tool's script with the given arguments
func (e *Executor) Execute(ctx context.Context, tool *config.Tool, args map[string]interface{}) *Result {
	start := time.Now()

	// Parse timeout
	timeout := 30 * time.Second
	if tool.Script.Timeout != "" {
		if d, err := time.ParseDuration(tool.Script.Timeout); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd

	if tool.Script.Shell != "" {
		// Shell mode - execute inline script
		shell, shellArg := getShell()
		script := expandArgs(tool.Script.Shell, args)
		cmd = exec.CommandContext(ctx, shell, shellArg, script)
	} else {
		// Command mode - execute command with args
		cmdArgs := make([]string, len(tool.Script.Args))
		for i, arg := range tool.Script.Args {
			cmdArgs[i] = expandArgs(arg, args)
		}
		cmd = exec.CommandContext(ctx, tool.Script.Command, cmdArgs...)
	}

	// Set working directory
	if tool.Script.WorkingDir != "" {
		cmd.Dir = os.ExpandEnv(tool.Script.WorkingDir)
	}

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range tool.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, os.ExpandEnv(v)))
	}

	// Add args as environment variables
	for k, v := range args {
		envKey := fmt.Sprintf("GANTZ_ARG_%s", strings.ToUpper(k))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%v", envKey, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &Result{
		Duration: time.Since(start),
	}

	// Combine stdout and stderr
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}
	result.Output = strings.TrimSpace(output)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Error = err
	}

	return result
}

// getShell returns the appropriate shell for the OS
func getShell() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/c"
	}
	// Try to use user's shell, fallback to sh
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, "-c"
}

// expandArgs replaces {{arg}} placeholders with actual values
func expandArgs(template string, args map[string]interface{}) string {
	result := template
	for k, v := range args {
		placeholder := fmt.Sprintf("{{%s}}", k)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}
	return result
}
