package runtime

type Codex struct{}

func (Codex) Command(r Request) Command {
	args := []string{"exec", "--ephemeral", "--color", "never", "--skip-git-repo-check"}
	if r.Model != "" && r.Model != "default" {
		args = append(args, "--model", r.Model)
	}
	args = append(args, "-")
	return Command{Executable: "codex", Args: args, Stdin: r.Prompt}
}
