package gha

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"

	"github.com/risor-io/risor/internal/arg"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/os"
)

func IsDebug(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("gha.is_debug", 0, args); err != nil {
		return err
	}
	env, ok := os.GetDefaultOS(ctx).LookupEnv("RUNNER_DEBUG")
	if !ok {
		return object.False
	}
	if env != "1" {
		return object.False
	}
	return object.True
}

func printableValue(obj object.Object) any {
	if iface := obj.Interface(); iface != nil {
		return iface
	}
	switch obj := obj.(type) {
	case fmt.Stringer:
		return obj.String()
	default:
		return obj.Inspect()
	}
}

var workflowCommandReplacer = strings.NewReplacer(
	// Trick to get newlines included
	// https://github.com/actions/toolkit/issues/193#issuecomment-605394935
	"\r\n", "%0A",
	"\r", "%0A",
	"\n", "%0A",
	"::", "%3A%3A",
)

func sanitizeCommandValue(value string) string {
	return workflowCommandReplacer.Replace(strings.TrimSuffix(value, "\n"))
}

func asCommandProps(obj *object.Map) map[string]any {
	props := make(map[string]any)
	for _, key := range obj.StringKeys() {
		cmdKey, ok := commandPropKey(key)
		if !ok {
			continue
		}
		props[cmdKey] = printableValue(obj.Get(key))
	}
	return props
}

func commandPropKey(key string) (string, bool) {
	switch key {
	case "title", "file", "line":
		return key, true
	case "column":
		return "col", true
	case "end_line":
		return "endLine", true
	case "end_column":
		return "endColumn", true
	default:
		return "", false
	}
}

func stringifyCommandProps(props map[string]any) string {
	var sb strings.Builder
	for key, value := range props {
		if sb.Len() == 0 {
			sb.WriteByte(' ')
		} else {
			sb.WriteByte(',')
		}
		sb.WriteString(key)
		sb.WriteByte('=')
		sb.WriteString(sanitizeCommandValue(fmt.Sprint(value)))
	}
	return sb.String()
}

func runWorkflowCommand(w io.Writer, cmd string, value any, props map[string]any) object.Object {
	if _, err := fmt.Fprintf(w, "::%s%s::%s\n",
		cmd,
		stringifyCommandProps(props),
		sanitizeCommandValue(fmt.Sprint(value))); err != nil {
		return object.Errorf("io error: %v", err)
	}
	return object.Nil
}

func appendWorkflowFile(ros os.OS, path string, message string) object.Object {
	file, err := ros.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return object.Errorf("io error: %v", err)
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, strings.TrimSuffix(message, "\n")); err != nil {
		return object.Errorf("io error: %v", err)
	}
	return object.Nil
}

func workflowFileKeyValue(key, value any) string {
	keyStr := fmt.Sprint(key)
	valueStr := fmt.Sprint(value)
	eof := workflowFileKeyValueEOF(keyStr, valueStr)
	return fmt.Sprintf("%s<<%s\n%s\n%s", keyStr, eof, valueStr, eof)
}

func workflowFileKeyValueEOF(key, value string) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var eof = "EOF"
	for strings.Contains(key, eof) || strings.Contains(value, eof) {
		eof += string(charset[rand.Intn(len(charset))])
	}
	return eof
}

func LogDebug(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("gha.log_debug", 1, args); err != nil {
		return err
	}
	message := printableValue(args[0])
	stdout := os.GetDefaultOS(ctx).Stdout()
	return runWorkflowCommand(stdout, "debug", message, nil)
}

func LogNotice(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("gha.log_notice", 1, 2, args); err != nil {
		return err
	}
	message := printableValue(args[0])
	var props map[string]any
	if len(args) > 1 {
		m, err := object.AsMap(args[1])
		if err != nil {
			return err
		}
		props = asCommandProps(m)
	}
	stdout := os.GetDefaultOS(ctx).Stdout()
	return runWorkflowCommand(stdout, "notice", message, props)
}

func LogWarning(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("gha.log_warning", 1, 2, args); err != nil {
		return err
	}
	message := printableValue(args[0])
	var props map[string]any
	if len(args) > 1 {
		m, err := object.AsMap(args[1])
		if err != nil {
			return err
		}
		props = asCommandProps(m)
	}
	stdout := os.GetDefaultOS(ctx).Stdout()
	return runWorkflowCommand(stdout, "warning", message, props)
}

func LogError(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.RequireRange("gha.log_error", 1, 2, args); err != nil {
		return err
	}
	message := printableValue(args[0])
	var props map[string]any
	if len(args) > 1 {
		m, err := object.AsMap(args[1])
		if err != nil {
			return err
		}
		props = asCommandProps(m)
	}
	stdout := os.GetDefaultOS(ctx).Stdout()
	return runWorkflowCommand(stdout, "error", message, props)
}

func StartGroup(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("gha.start_group", 1, args); err != nil {
		return err
	}
	message := printableValue(args[0])
	stdout := os.GetDefaultOS(ctx).Stdout()
	return runWorkflowCommand(stdout, "group", message, nil)
}

func EndGroup(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("gha.end_group", 0, args); err != nil {
		return err
	}
	stdout := os.GetDefaultOS(ctx).Stdout()
	return runWorkflowCommand(stdout, "endgroup", "", nil)
}

func SetOutput(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("gha.set_output", 2, args); err != nil {
		return err
	}
	key := printableValue(args[0])
	value := printableValue(args[1])

	ros := os.GetDefaultOS(ctx)
	outputFile := ros.Getenv("GITHUB_OUTPUT")
	if outputFile != "" {
		return appendWorkflowFile(ros, outputFile, workflowFileKeyValue(key, value))
	}

	stdout := ros.Stdout()
	// Using "::set-output::" command is deprecated, but it's a good enough fallback
	return runWorkflowCommand(stdout, "set-output", value, map[string]any{"name": key})
}

func SetEnv(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("gha.set_env", 2, args); err != nil {
		return err
	}
	key := fmt.Sprint(printableValue(args[0]))
	value := fmt.Sprint(printableValue(args[1]))

	ros := os.GetDefaultOS(ctx)
	ros.Setenv(key, value)

	envFile := ros.Getenv("GITHUB_ENV")
	if envFile != "" {
		return appendWorkflowFile(ros, envFile, workflowFileKeyValue(key, value))
	}

	stdout := ros.Stdout()
	// Using "::set-env::" command is deprecated, but it's a good enough fallback
	return runWorkflowCommand(stdout, "set-env", value, map[string]any{"name": key})
}

func AddPath(ctx context.Context, args ...object.Object) object.Object {
	if err := arg.Require("gha.add_path", 1, args); err != nil {
		return err
	}
	path := printableValue(args[0])
	pathStr := fmt.Sprint(path)

	ros := os.GetDefaultOS(ctx)
	oldPath := ros.Getenv("PATH")
	ros.Setenv("PATH", fmt.Sprintf("%s%c%s", pathStr, ros.PathListSeparator(), oldPath))

	pathFile := ros.Getenv("GITHUB_PATH")
	if pathFile != "" {
		return appendWorkflowFile(ros, pathFile, pathStr)
	}

	stdout := ros.Stdout()
	// Using "::add-path::" command is deprecated, but it's a good enough fallback
	return runWorkflowCommand(stdout, "add-path", path, nil)
}

func Module() *object.Module {
	return object.NewBuiltinsModule("gha", map[string]object.Object{
		"is_debug":    object.NewBuiltin("gha.is_debug", IsDebug),
		"log_debug":   object.NewBuiltin("gha.log_debug", LogDebug),
		"log_notice":  object.NewBuiltin("gha.log_notice", LogNotice),
		"log_warning": object.NewBuiltin("gha.log_warning", LogWarning),
		"log_error":   object.NewBuiltin("gha.log_error", LogError),
		"start_group": object.NewBuiltin("gha.start_group", StartGroup),
		"end_group":   object.NewBuiltin("gha.end_group", EndGroup),
		"set_output":  object.NewBuiltin("gha.set_output", SetOutput),
		"set_env":     object.NewBuiltin("gha.set_env", SetEnv),
		"add_path":    object.NewBuiltin("gha.add_path", AddPath),
	})
}