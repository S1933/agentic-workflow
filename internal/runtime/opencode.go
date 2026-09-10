package runtime

type OpenCode struct{}

func (OpenCode) Command(r Request) Command {
	args := []string{"run"}
	if r.Model != "" && r.Model != "default" {
		args = append(args, "--model", r.Model)
	}
	// The positional message is a single argument, never shell code.
	args = append(args, "--", r.Prompt)
	return Command{Executable: "opencode", Args: args}
}
