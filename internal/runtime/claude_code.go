package runtime

type ClaudeCode struct{}

func (ClaudeCode) Command(r Request) Command {
	args := []string{"-p", "--output-format", "text", "--no-session-persistence"}
	if r.Model != "" && r.Model != "default" {
		args = append(args, "--model", r.Model)
	}
	return Command{Executable: "claude", Args: args, Stdin: r.Prompt}
}
