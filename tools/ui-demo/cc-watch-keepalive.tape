Output docs/images/cc-watch-keepalive.gif

Set Shell zsh
Set FontFamily "JetBrains Mono"
Set FontSize 28
Set LetterSpacing 0
Set Width 1600
Set Height 1600
Set Padding 30
Set Margin 24
Set MarginFill "#16161e"
Set WindowBar Colorful
Set BorderRadius 10
Set Theme "Catppuccin Mocha"
Set CursorBlink false
Set Framerate 30
Set PlaybackSpeed 1.25
Set LoopOffset 1

Require go

# Start in Session Info with KeepAlive off.
Hide
Type `function cc-watch { sleep 1; clear; TERM=xterm-256color COLORTERM=truecolor CLICOLOR_FORCE=1 NO_COLOR= CC_WATCH_DEMO_CUES=off CC_WATCH_DEMO_KEEPALIVE=off CC_WATCH_DEMO_REMINDER=off CC_WATCH_DEMO_START=workspace go run -tags demo ./tools/ui-demo; }`
Enter
Type `clear`
Enter
Type `cc-watch`
Enter
Sleep 4s
Show

# Arm KeepAlive, jump to its trigger, and advance the fake clock to a send.
Sleep 2s
Type "k"
Sleep 2s
Type "j"
Sleep 2s
Type "."
Sleep 500ms
Type "."
Sleep 500ms
Type "."
Sleep 500ms
Type "."
Sleep 500ms
Type "."
Sleep 500ms
Type "."
Sleep 4s
