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

	ModeTUI            Mode = "tui"
	ModeHelp           Mode = "help"
	ModeVersion        Mode = "version"
	ModeConfig         Mode = "config"
	ModeStatusline     Mode = "statusline"
	ModeStatuslineHelp Mode = "statusline_help"
)

const statuslineUsageError = "statusline: invalid arguments, expected one of: `cc-watch statusline`, `cc-watch statusline -- <command>`, `cc-watch statusline --check`, `cc-watch statusline --help`"

type Command struct {
	Mode           Mode
	Limit          int
	ID             string
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
		case rest[0] == "--help" || rest[0] == "-h":
			if len(rest) > 1 {
				return cmd, errors.New(statuslineUsageError)
			}
			cmd.Mode = ModeStatuslineHelp
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
	return cmd, nil
}

func WriteHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  cc-watch [--n N] [--id <partial-id>]
  cc-watch config
  cc-watch statusline
  cc-watch statusline -- <command> [args...]
  cc-watch statusline --check
  cc-watch statusline --help
  cc-watch --help
  cc-watch --version

TUI:
  cc-watch                  Open recent Claude Code sessions.
  cc-watch --n 10           Load 10 recent sessions.
  cc-watch --id d4b247b7    Open the matching session.
  cc-watch config           Edit Reminder, KeepAlive, and Statusline settings.

Statusline:
  See: cc-watch statusline --help

Safety:
  Reminder and KeepAlive are controlled inside the TUI.
  statusline --check never edits Claude Code settings.
  statusline runtime exits 0 so it does not break Claude Code's UI.

Examples:
  cc-watch
  cc-watch --id d4b247b7
  cc-watch config
  cc-watch statusline --check
  cc-watch statusline -- ~/.claude/statusline.sh
`)
}

func WriteStatuslineHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  cc-watch statusline
  cc-watch statusline -- <command> [args...]
  cc-watch statusline --check

Modes:
  cc-watch statusline
      Read Claude Code statusline JSON from stdin and print cc-watch's segment,
      for example: ⏱ 34% (5h) / 41% (7d) used.

  cc-watch statusline -- <command> [args...]
      Run an existing statusline command, then append cc-watch after " | ".

  cc-watch statusline --check
      Read ~/.claude/settings.json and print install/uninstall guidance.
      It writes nothing.

Warning:
  "KeepAlive at risk" means a 5h or 7d Claude Code account limit may block
  future KeepAlive sends.

Examples:
  cc-watch statusline --check
  cc-watch statusline -- ~/.claude/statusline.sh
  cc-watch statusline -- sh -c 'echo "$USER"'
`)
}
