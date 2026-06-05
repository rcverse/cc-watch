package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
)

type Mode string

const (
	ModeTUI     Mode = "tui"
	ModeHelp    Mode = "help"
	ModeVersion Mode = "version"
	ModeJSON    Mode = "json"
	ModeConfig  Mode = "config"
)

type Command struct {
	Mode   Mode
	Limit  int
	ID     string
	JSON   bool
	Remind bool
}

func ParseArgs(args []string) (Command, error) {
	cmd := Command{
		Mode:  ModeTUI,
		Limit: 5,
	}

	if len(args) > 0 && args[0] == "config" {
		if len(args) > 1 {
			return cmd, fmt.Errorf("config does not accept arguments: %v", args[1:])
		}
		cmd.Mode = ModeConfig
		return cmd, nil
	}

	fs := flag.NewFlagSet("cc-cache", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var help bool
	var version bool
	var watch bool
	fs.BoolVar(&help, "help", false, "show help")
	fs.BoolVar(&help, "h", false, "show help")
	fs.BoolVar(&version, "version", false, "show version")
	fs.IntVar(&cmd.Limit, "n", cmd.Limit, "number of recent sessions")
	fs.StringVar(&cmd.ID, "id", "", "session id")
	fs.BoolVar(&cmd.JSON, "json", false, "machine-readable JSON")
	fs.BoolVar(&cmd.Remind, "remind", false, "enable reminders")
	fs.BoolVar(&watch, "watch", false, "unsupported")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			cmd.Mode = ModeHelp
			return cmd, nil
		}
		return cmd, err
	}
	if fs.NArg() > 0 {
		return cmd, fmt.Errorf("unexpected argument: %s", fs.Arg(0))
	}
	if help {
		cmd.Mode = ModeHelp
		return cmd, nil
	}
	if version {
		cmd.Mode = ModeVersion
		return cmd, nil
	}
	if watch {
		return cmd, errors.New("--watch is not part of cc-cache v2; live refresh is internal to the TUI")
	}
	if cmd.Limit < 1 {
		return cmd, fmt.Errorf("--n must be positive, got %s", strconv.Itoa(cmd.Limit))
	}
	if cmd.JSON {
		cmd.Mode = ModeJSON
	}

	return cmd, nil
}

func WriteHelp(w io.Writer) {
	fmt.Fprint(w, `Usage: cc-cache [--n N] [--id <partial-id>] [--json] [--remind]
       cc-cache config
       cc-cache --help
       cc-cache --version

Options:
  --n N              number of recent sessions to list
  --id <partial-id> open or output one session
  --json            machine-readable JSON, then exit
  --remind          start TUI with reminders enabled
  --help, -h        show help
  --version         show version

Unsupported:
  --watch           not part of cc-cache v2; live refresh is internal
`)
}
