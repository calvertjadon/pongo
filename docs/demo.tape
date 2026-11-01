# Records docs/demo.gif, the README demo. From the repository root, with VHS (https://github.com/charmbracelet/vhs)
# and its ttyd and ffmpeg dependencies installed:
#
#   vhs docs/demo.tape
#
# The match is scripted blind. The core is deterministic and the computer ignores the human paddle, so each paddle
# move only has to land between the human's previous contact and the next one. The sleeps below place every move in
# the middle of its window, which leaves about 0.75 seconds of slack either way. The planned match on Normal, in
# 1/120-second steps after Start match:
#
#   steps 0-736      1-0  W W at 236 angles the first return past the computer
#   steps 736-2139   1-1  the player returns twice (S x4 at 960, S x2 at 1471), then misses (W x3 at 1898)
#   steps 2139-2644  2-1  W x4 at 2237 wins the point
#   step 2900        pause mid-rally
#
# Changing the setup keys, the difficulty, the court geometry, ball physics, or the computer player invalidates the
# plan. Re-derive the windows from the core before re-recording.

Output docs/demo.gif

Set Shell bash
Set FontSize 16
Set Width 960
Set Height 640
Set Padding 20
Set TypingSpeed 100ms

Hide
Type "go build -o /tmp/pongo-demo/pongo . && export PATH=/tmp/pongo-demo:$PATH && clear"
Enter
Sleep 5s
Show

Type "pongo"
Sleep 300ms
Enter
Sleep 1.5s

# Match setup: winning score 5, Normal difficulty, Start match.
Enter
Sleep 1s
Down
Sleep 500ms
Left
Sleep 800ms
Down
Sleep 500ms
Down
Sleep 700ms
Enter

# Match. Each Type costs 100ms per key; the sleeps account for it.
Sleep 1.87s
Type "ww"
Sleep 5.83s
Type "ssss"
Sleep 3.86s
Type "ss"
Sleep 3.36s
Type "www"
Sleep 2.53s
Type "wwww"
Sleep 5.13s
Type "p"

# Pause menu: Return to title, then quit.
Sleep 2s
Down
Sleep 500ms
Down
Sleep 700ms
Enter
Sleep 1.5s
Type "q"
Sleep 1s
