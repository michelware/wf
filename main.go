// @Author: F.Michel
// @github: https://github.com/phpdevcommunity
package main

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/urfave/cli/v2"
)

var (
	version              = "2.1.0"
	workflows            []Workflow
	dockerComposeCommand = ""
	workflowDepth        = 0
	logFile              = ""
	runSection           = ""
	runStart             time.Time
	currentLine          = ""
)

func main() {
	currentDir, _ := os.Getwd()
	wfFiles, _ := filepath.Glob(filepath.Join(currentDir, "*.wf"))

	if len(wfFiles) == 0 {
		pterm.Warning.Println("No workflow files found")
	}

	workflows = []Workflow{}
	for _, wfFile := range wfFiles {
		for _, wf := range ParseContentToWorkFlowStruct(wfFile) {
			if len(wf.Lines) > 0 {
				workflows = append(workflows, wf)
			}
		}
	}

	Commands := []*cli.Command{}
	values := InitDefaultVariables()
	for _, wf := range workflows {
		Commands = append(Commands, &cli.Command{
			Name:  wf.Name,
			Usage: wf.Comment,
			Action: func(c *cli.Context) error {
				runSection = wf.Name
				runStart = time.Now()
				logFile = wf.LogFile
				executeWorkflow(wf, values)
				writeLog("SUCCESS")
				return nil
			},
		})
	}

	if len(os.Args) <= 1 {
		pterm.DefaultBigText.WithLetters(
			pterm.NewLettersFromStringWithStyle("W", pterm.NewStyle(pterm.FgCyan)),
			pterm.NewLettersFromStringWithStyle("F", pterm.NewStyle(pterm.FgBlue)),
		).Render()
	}

	app := &cli.App{
		Name:     "wf",
		Usage:    "run the workflows of the .wf files in the current folder",
		Version:  version,
		Commands: Commands,
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func executeWorkflow(wf Workflow, values map[string]string) {
	fmt.Println()
	start := time.Now()

	depth := workflowDepth
	workflowDepth++
	defer func() { workflowDepth-- }()

	indent := ""
	if depth > 0 {
		indent = strings.Repeat("  ", depth)
	}

	if depth == 0 {
		pterm.DefaultHeader.
			WithFullWidth().
			WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).
			WithMargin(1).
			Printfln("🚀 WORKFLOW: %s", strings.ToUpper(wf.Name))
		if logFile != "" {
			printLine(pterm.Info.Sprintf("This run is logged to %s", logFile))
		}
	} else {
		pterm.FgYellow.Printfln("%s╭── 📦 Sub-Workflow: %s", indent, wf.Name)
	}

	for _, line := range wf.Lines {
		executeLine(line, values)
	}

	fmt.Println()
	elapsed := time.Since(start).Round(time.Millisecond)
	if depth == 0 {
		pterm.Success.Printfln("✨ Workflow '%s' completed in %s.", wf.Name, elapsed)
		fmt.Println()
	} else {
		pterm.FgYellow.Printfln("%s╰── ✔ Completed in %s", indent, elapsed)
	}
}

func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || !os.IsNotExist(err)
}

func Touch(filename string) {
	fd, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		fatal("Failed to touch file %s: %v", filename, err)
	}
	fd.Close()
}

func Copy(source string, dest string) {
	fd1, err := os.Open(source)
	if err != nil {
		fatal("Failed to open source file %s: %v", source, err)
	}
	defer fd1.Close()

	fd2, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fatal("Failed to create destination file %s: %v", dest, err)
	}
	defer fd2.Close()

	if _, err := io.Copy(fd2, fd1); err != nil {
		fatal("Failed to copy %s to %s: %v", source, dest, err)
	}
}

func executeLine(lineOriginal string, values map[string]string) {
	if lineOriginal == "" || strings.HasPrefix(lineOriginal, "#") {
		return
	}

	currentLine = lineOriginal
	lineOriginal = ResolveVariables(values, lineOriginal)
	actionOriginal := strings.Split(lineOriginal, " ")[0]
	action := strings.ToLower(actionOriginal)
	line := strings.Replace(lineOriginal, actionOriginal, action, 1)

	switch action {
	case "set":
		name, value, _ := strings.Cut(strings.TrimPrefix(line, "set "), "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if name == "" || value == "" {
			fatal("Invalid set command")
		}
		values[name] = value
		printLine(pterm.Info.Sprintf("Variable %s set to %s", name, value))

	case "run":
		run(strings.TrimPrefix(line, "run "), true)

	case "echo":
		msg := strings.TrimSpace(strings.TrimPrefix(line, "echo "))
		if strings.HasPrefix(msg, "\"") && strings.HasSuffix(msg, "\"") {
			msg = msg[1 : len(msg)-1]
		}
		printLine(pterm.DefaultBasicText.Sprint("  " + msg))

	case "exit":
		args := strings.Fields(line)
		code := 0
		if len(args) > 1 {
			var err error
			code, err = strconv.Atoi(args[1])
			if err != nil {
				fatal("Invalid exit code: %s", args[1])
			}
		}
		writeLog(fmt.Sprintf("EXIT %d", code))
		os.Exit(code)

	case "touch":
		fileToCreate := strings.TrimSpace(strings.TrimPrefix(line, "touch "))
		if fileToCreate == "" {
			fatal("Invalid touch command")
		}
		if FileExists(fileToCreate) {
			printLine(pterm.Info.Sprintf("%s already exists, skipping...", fileToCreate))
			return
		}
		Touch(fileToCreate)
		printLine(pterm.Success.Sprintf("%s created", fileToCreate))

	case "copy", "cp":
		parts := strings.Fields(line)
		if len(parts) < 3 {
			fatal("Invalid copy command")
		}
		fileOrFolder, destination := parts[1], parts[2]
		if !FileExists(fileOrFolder) {
			fatal("%s not found", fileOrFolder)
		}
		if FileExists(destination) {
			printLine(pterm.Info.Sprintf("%s already exists, skipping...", destination))
			return
		}
		Copy(fileOrFolder, destination)
		printLine(pterm.Success.Sprintf("%s copied to %s", fileOrFolder, destination))

	case "mkdir":
		parts := strings.Fields(line)
		if len(parts) < 2 {
			fatal("Invalid mkdir command")
		}
		folderName := parts[1]
		if FileExists(folderName) {
			printLine(pterm.Info.Sprintf("Folder %s already exists, skipping...", folderName))
			return
		}
		if err := os.MkdirAll(folderName, os.ModePerm); err != nil {
			fatal(err.Error())
		}
		printLine(pterm.Success.Sprintf("Folder '%s' created", folderName))

	case "set_permissions":
		args := strings.Fields(line)
		if len(args) < 3 {
			fatal("Invalid set_permissions command: %s", line)
		}
		folderName := args[1]
		permissions, err := strconv.ParseUint(args[2], 8, 32)
		if err != nil {
			fatal("Invalid permissions: %s", args[2])
		}
		fi, err := os.Stat(folderName)
		if err != nil {
			fatal("Failed to access '%s': %s", folderName, err)
		}
		err = filepath.Walk(folderName, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			return os.Chmod(path, os.FileMode(permissions))
		})
		if err != nil {
			fatal("Failed to set permissions on '%s': %s", folderName, err)
		}
		if fi.IsDir() {
			printLine(pterm.Info.Sprintf("Permissions 0%o applied recursively to '%s'", permissions, folderName))
		} else {
			printLine(pterm.Info.Sprintf("Permissions 0%o applied to '%s'", permissions, folderName))
		}

	case "sync_time":
		printLine(pterm.Info.Sprintf("System time: %s", time.Now().Format("2006-01-02 15:04:05 MST")))

	case "docker_compose":
		args := strings.Fields(line)
		if len(args) < 2 {
			fatal("Invalid execute command: %s", line)
		}
		run(fmt.Sprintf("%s %s", GetDockerComposeCommand(), strings.Join(args[1:], " ")), true)

	case "wf":
		args := strings.Fields(line)
		if len(args) < 2 {
			fatal("Invalid wf command: %s", line)
		}
		wfName := args[1]
		for _, wf := range workflows {
			if wf.Name == wfName {
				executeWorkflow(wf, values)
				return
			}
		}
		fatal("Workflow '%s' not found", wfName)

	case "notify_success", "notify_error", "notify_warning", "notify_info":
		notifies := map[string]*pterm.PrefixPrinter{
			"notify_success": &pterm.Success,
			"notify_error":   &pterm.Error,
			"notify_warning": &pterm.Warning,
			"notify_info":    &pterm.Info,
		}
		printer := notifies[action]
		msg := strings.TrimSpace(strings.TrimPrefix(line, action))
		if strings.HasPrefix(msg, "\"") && strings.HasSuffix(msg, "\"") {
			msg = msg[1 : len(msg)-1]
		}
		printLine(printer.Sprint(msg))

	case "notify":
		msg := strings.TrimSpace(strings.TrimPrefix(line, "notify"))
		if strings.HasPrefix(msg, "\"") && strings.HasSuffix(msg, "\"") {
			msg = msg[1 : len(msg)-1]
		}
		printLine(pterm.DefaultBasicText.Sprint("  " + msg))

	case "write":
		rest := strings.TrimSpace(strings.TrimPrefix(line, "write"))
		file, text, _ := strings.Cut(rest, " ")
		if file == "" {
			fatal("Invalid write command: %s", line)
		}
		fd, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fatal("Failed to open %s: %v", file, err)
		}
		_, err = fd.WriteString(unquote(strings.TrimSpace(text)) + "\n")
		fd.Close()
		if err != nil {
			fatal("Failed to write to %s: %v", file, err)
		}
		printLine(pterm.Success.Sprintf("Line written to %s", file))

	case "remove":
		args := strings.Fields(line)
		if len(args) < 2 {
			fatal("Invalid remove command: %s", line)
		}
		target := args[1]
		absTarget, _ := filepath.Abs(target)
		if absTarget == getCurrentDir() || filepath.Dir(absTarget) == absTarget {
			fatal("Refusing to remove '%s'", target)
		}
		if !FileExists(target) {
			printLine(pterm.Info.Sprintf("%s does not exist, skipping...", target))
			return
		}
		if err := os.RemoveAll(target); err != nil {
			fatal("Failed to remove %s: %v", target, err)
		}
		printLine(pterm.Success.Sprintf("%s removed", target))

	case "move":
		args := strings.Fields(line)
		if len(args) < 3 {
			fatal("Invalid move command: %s", line)
		}
		source, destination := args[1], args[2]
		if !FileExists(source) {
			fatal("%s not found", source)
		}
		if FileExists(destination) {
			printLine(pterm.Info.Sprintf("%s already exists, skipping...", destination))
			return
		}
		if err := os.Rename(source, destination); err != nil {
			fatal("Failed to move %s to %s: %v", source, destination, err)
		}
		printLine(pterm.Success.Sprintf("%s moved to %s", source, destination))

	case "require":
		args := strings.Fields(line)
		if len(args) < 2 {
			fatal("Invalid require command: %s", line)
		}
		for _, program := range args[1:] {
			if _, err := exec.LookPath(program); err != nil {
				fatal("%s is required but was not found in PATH", program)
			}
			printLine(pterm.Success.Sprintf("%s found", program))
		}

	case "sleep":
		args := strings.Fields(line)
		if len(args) < 2 {
			fatal("Invalid sleep command: %s", line)
		}
		seconds, err := strconv.Atoi(args[1])
		if err != nil || seconds < 0 {
			fatal("Invalid sleep duration: %s", args[1])
		}
		printLine(pterm.Info.Sprintf("Waiting %ds...", seconds))
		time.Sleep(time.Duration(seconds) * time.Second)

	case "confirm":
		question := unquote(strings.TrimSpace(strings.TrimPrefix(line, "confirm")))
		if workflowDepth > 1 {
			fmt.Print(strings.Repeat("  ", workflowDepth-1))
		}
		fmt.Print(pterm.Warning.Sprint(question) + " [y/N] ")
		answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println()
			printLine(pterm.Error.Sprint("Cancelled"))
			writeLog("CANCELLED")
			os.Exit(1)
		}

	case "load_env":
		args := strings.Fields(line)
		if len(args) < 2 {
			fatal("Invalid load_env command: %s", line)
		}
		content, err := os.ReadFile(args[1])
		if err != nil {
			fatal("Failed to read %s: %v", args[1], err)
		}
		count := 0
		for _, envLine := range strings.Split(string(content), "\n") {
			envLine = strings.TrimSpace(envLine)
			if envLine == "" || strings.HasPrefix(envLine, "#") {
				continue
			}
			name, value, found := strings.Cut(envLine, "=")
			if !found {
				continue
			}
			values[strings.TrimSpace(name)] = unquote(strings.TrimSpace(value))
			count++
		}
		printLine(pterm.Info.Sprintf("%d variables loaded from %s", count, args[1]))

	default:
		fatal("Unknown command or invalid syntax : " + line)
	}
}

func run(line string, printCommand bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	args := splitArgs(line)

	var start time.Time
	indent := ""
	if workflowDepth > 1 {
		indent = strings.Repeat("  ", workflowDepth-1)
	}

	if printCommand {
		fmt.Println()
		pterm.FgCyan.Printfln("%s╭─ ▶ %s", indent, line)
		start = time.Now()
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()

	if printCommand {
		prefixStr := pterm.FgCyan.Sprint(indent + "│  ")
		cmd.Stdout = NewPrefixWriter(prefixStr, os.Stdout)
		cmd.Stderr = NewPrefixWriter(prefixStr, os.Stderr)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()

	if printCommand {
		elapsed := time.Since(start).Round(time.Millisecond)
		if err != nil {
			pterm.FgRed.Printfln("%s╰─ ✘ Failed in %s", indent, elapsed)
		} else {
			pterm.FgGreen.Printfln("%s╰─ ✔ %s", indent, elapsed)
		}
	}

	if err != nil {
		fmt.Println()
		pterm.Error.Printfln("Command failed '%s': %v", line, err)
		writeLog("FAILED")
		os.Exit(1)
	}
}

// ParseContentToWorkFlowStruct returns the sections of a .wf file in the
// order they are written. Lines before the first section go to "main".
func ParseContentToWorkFlowStruct(filename string) []Workflow {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	fileWorkflows := []Workflow{{Name: "main"}}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") {
			end := strings.Index(line, "]")
			if end == -1 {
				continue
			}
			wfName := line[1:end]
			options, comment, _ := strings.Cut(line[end+1:], "#")
			comment = strings.TrimSpace(comment)

			// Section options sit between "]" and the comment, as key=value.
			// The only option is log=<file>; anything else stops WF.
			logFile := ""
			for _, option := range strings.Fields(options) {
				key, value, _ := strings.Cut(option, "=")
				if key != "log" {
					fatal("Unknown section option '%s' in [%s]", option, wfName)
				}
				if value == "" {
					fatal("Invalid section header [%s]: log needs a file (log=<file>)", wfName)
				}
				if logFile != "" {
					fatal("Invalid section header [%s]: log is set twice", wfName)
				}
				logFile = value
			}

			fileWorkflows = append(fileWorkflows, Workflow{Name: wfName, Comment: comment, LogFile: logFile})
			continue
		}

		last := len(fileWorkflows) - 1
		fileWorkflows[last].Lines = append(fileWorkflows[last].Lines, line)
	}
	return fileWorkflows
}

func InitDefaultVariables() map[string]string {
	return map[string]string{
		"IP_LOCAL":     GetLocalIP(),
		"CURRENT_PATH": getCurrentDir(),
	}
}

func getCurrentDir() string {
	dir, _ := os.Getwd()
	return dir
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return ""
}

func TokenGenerator(size int) string {
	b := make([]byte, size)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func ResolveVariables(values map[string]string, line string) string {
	for key, value := range values {
		line = strings.ReplaceAll(line, "${"+key+"}", value)
	}
	return strings.ReplaceAll(line, "${GENERATE_SECRET}", TokenGenerator(32))
}

func GetDockerComposeCommand() string {
	if dockerComposeCommand == "" {
		if err := exec.Command("docker", "compose", "--version").Run(); err != nil {
			dockerComposeCommand = "docker-compose"
		} else {
			dockerComposeCommand = "docker compose"
		}
	}
	return dockerComposeCommand
}

type Workflow struct {
	Name    string
	Comment string
	LogFile string
	Lines   []string
}

type PrefixWriter struct {
	Prefix []byte
	Writer io.Writer
	isBOL  bool
}

func NewPrefixWriter(prefix string, writer io.Writer) *PrefixWriter {
	return &PrefixWriter{
		Prefix: []byte(prefix),
		Writer: writer,
		isBOL:  true,
	}
}

func (pw *PrefixWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if pw.isBOL {
			if _, err = pw.Writer.Write(pw.Prefix); err != nil {
				return n, err
			}
			pw.isBOL = false
		}
		if _, err = pw.Writer.Write([]byte{b}); err != nil {
			return n, err
		}
		n++
		if b == '\n' {
			pw.isBOL = true
		}
	}
	return n, nil
}

func printLine(text string) {
	if workflowDepth > 1 {
		fmt.Print(strings.Repeat("  ", workflowDepth-1))
	}
	fmt.Println(text)
}

// splitArgs splits a command line on spaces, like a terminal does:
// "..." and '...' group words anywhere in an argument, and are removed.
// Backslashes, pipes and $VAR are plain text: there is no shell.
func splitArgs(line string) []string {
	args := []string{}
	current := ""
	inArg := false
	var quote rune
	for _, c := range line {
		switch {
		case quote != 0 && c == quote:
			quote = 0
		case quote != 0:
			current += string(c)
		case c == '"' || c == '\'':
			quote = c
			inArg = true
		case c == ' ' || c == '\t':
			if inArg {
				args = append(args, current)
				current, inArg = "", false
			}
		default:
			current += string(c)
			inArg = true
		}
	}
	if quote != 0 {
		fatal("Missing closing quote in: %s", line)
	}
	if inArg {
		args = append(args, current)
	}
	return args
}

// writeLog appends one line about the current run to the file set by the
// "log" command, and does nothing when the workflow has no "log" line.
// A failed run also logs the .wf line that failed, as written in the file:
// variables are not replaced, so no secret ends up in the log.
func writeLog(status string) {
	if logFile == "" {
		return
	}
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	host, _ := os.Hostname()
	entry := fmt.Sprintf("%s | %-15s | %-9s | %-7s | %s@%s",
		runStart.Format("2006-01-02 15:04:05"),
		runSection,
		status,
		time.Since(runStart).Round(100*time.Millisecond).String(),
		user,
		host,
	)
	if status == "FAILED" {
		entry += " | " + currentLine
	}

	fd, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		pterm.Warning.Printfln("Failed to write log %s: %v", logFile, err)
		return
	}
	fd.WriteString(entry + "\n")
	fd.Close()
}

// unquote removes one pair of surrounding double or single quotes.
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func fatal(format string, a ...interface{}) {
	printLine(pterm.Error.Sprintf(format, a...))
	writeLog("FAILED")
	os.Exit(1)
}
