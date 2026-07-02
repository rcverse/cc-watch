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
	DefaultLimit = 25

	ModeTUI        Mode = "tui"
	ModeHelp       Mode = "help"
	ModeVersion    Mode = "version"
	ModeJSON       Mode = "json"
	ModeConfig     Mode = "config"
	ModeStatusline Mode = "statusline"
)

const statuslineUsageError = "statusline: invalid arguments, expected one of: `cc-watch statusline`, `cc-watch statusline -- <command>`, `cc-watch statusline --check`"

type Command struct {
	Mode           Mode
	Limit          int
	ID             string
	JSON           bool
	Remind         bool
	WrappedCommand []string
	CheckConfig    bool
}

func ParseArgs(args []string) (Command, error) {
	cmd := Command{
		Mode:  ModeTUI,
		Limit: DefaultLimit,
	}

	if len(args) > 0 && args[0] == "config" {
		if len(args) > 1 {
			return cmd, fmt.Errorf("config does not accept arguments: %v", args[1:])
		}
		cmd.Mode = ModeConfig
		return cmd, nil
	}

	if len(args) > 0 && args[0] == "statusline" {
		cmd.Mode = ModeStatusline
		rest := args[1:]
		switch {
		case len(rest) == 0:
			return cmd, nil
		case rest[0] == "--check":
			if len(rest) > 1 {
				return cmd, errors.New(statuslineUsageError)
			}
			cmd.CheckConfig = true
			return cmd, nil
		case rest[0] == "--":
			if len(rest) < 2 {
				return cmd, fmt.Errorf("statusline: no command given after --")
			}
			cmd.WrappedCommand = append([]string(nil), rest[1:]...)
			return cmd, nil
		default:
			return cmd, errors.New(statuslineUsageError)
		}
	}

	fs := flag.NewFlagSet("cc-watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var help bool
	var version bool
	fs.BoolVar(&help, "help", false, "show help")
	fs.BoolVar(&help, "h", false, "show help")
	fs.BoolVar(&version, "version", false, "show version")
	fs.IntVar(&cmd.Limit, "n", cmd.Limit, "number of recent sessions")
	fs.StringVar(&cmd.ID, "id", "", "session id")
	fs.BoolVar(&cmd.JSON, "json", false, "machine-readable JSON")
	fs.BoolVar(&cmd.Remind, "remind", false, "enable reminders")

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
	if cmd.Limit < 1 {
		return cmd, fmt.Errorf("--n must be positive, got %s", strconv.Itoa(cmd.Limit))
	}
	if cmd.JSON {
		cmd.Mode = ModeJSON
	}

	return cmd, nil
}

func WriteHelp(w io.Writer) {
	fmt.Fprint(w, `Usage: cc-watch [--n N] [--id <partial-id>] [--json] [--remind]
       cc-watch config
       cc-watch statusline
       cc-watch statusline -- <command> [args...]
       cc-watch statusline --check
       cc-watch --help
       cc-watch --version

Options:
  --n N              load N recent sessions for the List View
  --id <partial-id> open or output one session
  --json            machine-readable JSON, then exit
  --remind          start TUI with reminders enabled
  --help, -h        show help
  --version         show version

statusline:
  Plugs into Claude Code's statusLine.command hook to append an estimated
  "messages left before the 5-hour rate limit resets" readout.
    cc-watch statusline               emit only cc-watch's own readout
    cc-watch statusline -- <command>  wrap an existing statusline command
    cc-watch statusline --check       read-only: report current wiring

Examples:
  cc-watch
  cc-watch --n 10
  cc-watch --id d4b247b7
  cc-watch --json --id d4b247b7
  cc-watch config
  cc-watch statusline --check
`)
}
