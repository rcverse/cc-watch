# cc-watch v1 Archive

This directory preserves the Python v1 implementation while cc-watch v2 is built in Go.

Files:

- `cc_watch.py` is an exact copy of the root v1 script at the time it was archived.
- `install-v1.sh` is an exact copy of the v1 installer.

The root `cc_watch.py` remains in place during v2 implementation because the installed command path is expected to point at it:

```text
$HOME/.local/bin/cc-watch -> /Users/richardchen/Dev/cc-watch/cc_watch.py
```

Do not replace that command path until the v2 binary has passed verification and the user explicitly approves switchover.

To run the archived v1 script directly:

```bash
python3 archive/v1/cc_watch.py --help
```
