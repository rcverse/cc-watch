Output docs/images/cc-watch-demo.gif

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

# Define a safe synthetic-data `cc-watch` command without recording setup.
Hide
Type `function cc-watch { sleep 1; clear; TERM=xterm-256color COLORTERM=truecolor CLICOLOR_FORCE=1 NO_COLOR= CC_WATCH_DEMO_CUES=off CC_WATCH_DEMO_KEEPALIVE=off CC_WATCH_DEMO_REMINDER=off go run -tags demo ./tools/ui-demo; }`
Enter
Type `clear`
Enter
Show
Type `cc-watch`
Enter
Sleep 4s

# Show the full list, switch to attention order, pin the selected session,
# then inspect its model and context details.
Sleep 500ms
Type "s"
Sleep 3s
Type "p"
Sleep 2s
Enter
Sleep 2s
Type "v"
Sleep 4s
