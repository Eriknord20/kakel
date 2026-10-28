# What kakel does

The whole list. The [README](README.md) has the short version.

## What works

- **A real terminal.** bash, vim and less all run: alternate screen,
  scroll regions, scrollback, 256 and true colour, bold, dim, italic,
  underline, strikethrough and reverse video, window title, cursor
  shapes, a cursor that blinks when the program asks for one, device
  reports, bracketed paste and mouse modes.
- **Local shells and SSH.** One `session.Session` interface with two
  implementations. Nothing above it — the emulator, the grid, the
  window — can tell the difference.
- **Profiles for the shells kakel does not find.** Settings › Terminal ›
  Profiles adds MSYS2, nushell or `ssh host` by its command line, as
  Windows Terminal does: `C:\msys64\usr\bin\bash.exe --login -i`.
  A profile may name a folder to start in and variables to set, one
  `NAME=value` a line; a value may use one set already, as
  `PATH=C:\tools;%PATH%` does, or `$PATH` on Linux and macOS. A name
  not set stays as written, and `%%` or `$$` writes one sign. A WSL
  profile carries them into the distribution, a Windows path
  translated. It is offered with the
  shells found here, in the palette, the split chooser, the launcher,
  the tray and the Shell setting, and can be the shell new terminals
  start. Renaming it keeps
  the shell chosen and the shortcuts bound to it. A profile on a saved
  server, such as `tmux new -A -s main` or `htop`, runs there in place
  of the account's shell, and the server's dialog can make it what New
  Terminal opens there; Login Shell on its menu still opens the plain
  shell. It needs a POSIX server: variables go through `env` and the
  folder through `cd`, and `.profile` is read only when the command
  asks for a login shell, as `bash -l` does.
- **Connections, not just shells.** One SSH connection carries several
  things at once, so a second terminal on a machine is a second channel
  rather than a second login. A command runs on a channel of the same
  connection, in a pane named by what it runs. Connect from inside the
  window with `Ctrl+Shift+N`.
- **One kakel window working in another.** A window can serve itself
  on a port you opt into, and another window on another machine can take
  it over: what it has open appears in the Machines pane under that
  window's name, and a pane
  opened there is drawn here. Key authentication only, from a list of
  keys you add in the Serve dialog, pasted or picked from this
  machine's; there is no password and no way past an unknown host
  key but saying yes to its fingerprint, and a host key that changed is a
  hard failure. The window being served keeps drawing and says who is
  working in it. Closing the connection gives it its screen back.
- **Through a relay, when neither can reach the other.** `kakel -relay`
  runs on a server with no screen. A window served through it gets a
  code, kept from one run to the next, and a window connecting gives the
  relay that code. Both reach it over WebSockets on HTTPS, which gets
  through HTTP proxies. The relay only carries bytes: the windows speak
  SSH through it, the keys allowed in are the same, and the host key is
  checked as for any window.
- **Tunnels for plain ssh.** `ssh -N -L`, `-J` and `-W` work against a
  served window, with the keys it allows. The user name picks where the
  tunnel goes out from: the served machine, or a machine saved there,
  connected to on the way when nothing has to be asked.
- **Working in a shell that is already running over there.** The served
  window says what it has open, and choosing one of those rows opens a
  pane here on the program that is already running there, starting with
  the screen as it stands. Both people see it and either can type. It
  keeps running over there when this window lets go. A pane already
  watching something comes forward rather than opening a second one, and
  the row says when somebody elsewhere is reading it.
- **The files of the window taken over.** The same connection carries
  them, as SFTP on a channel of its own, so a file manager pane on that
  machine costs no second login. A window that would rather not offer
  its files refuses the channel by name.
- **The machines beyond a connected window.** The servers that window
  is connected to get a heading each under its name, and their plus
  offers what a server of your own does: a terminal, a command, files
  and its saved folders, a tunnel or SOCKS proxy, its connection log, and
  Disconnect, which asks that window to close its connection. Each
  rides the one connection to the window. A tunnel through a window
  listens here; a remote forward needs a connection of your own. An
  agent handed a pane out there works in it as in any other.
- **Panes shared with an agent.** Set the panes up -- through whatever
  machines, as whatever user, with whatever credentials -- put them in a
  share, and give a program you are talking to the one code for it. The
  code is the whole of what lets it in, and with it the agent can read
  those panes, type into them, and wait for them to settle. It reaches
  no other pane, no connection of yours and no file except through the
  panes you shared. Add a pane while it works and it is there the next
  time the agent asks what it has; take one out and it is gone at once.
  So "set this up across three machines" is one code and one prompt. It
  is a narrow way in rather than a fence around what follows: what it
  types goes into live shells running as whoever you set those panes up
  as, and they do whatever those shells do — in your panes, in front of
  you, and you can take them back. Nothing listens until you share a
  pane, the port is on the loopback address, and taking the last pane
  back makes the code useless at once.
  `kakel -mcp` is the Model Context Protocol server the agent runs;
  it holds no credentials and reaches nothing until you give it a code.
- **One machine reached through another.** A saved server can say it is
  behind another one. The second connection is carried inside a channel
  of the first, so no local port is opened for it and nothing else on
  the machine can use it. Closing the one in the middle closes what
  rides on it. A saved server can be behind a kakel window too, served
  directly or through a relay: the window opens the stream from its own
  machine, so a server on that machine's network is reached from
  anywhere the window is.
- **Tabs.** Each window holds tabs, and each tab a pane or a split. New
  Terminal opens one, and the tab bar shows in the title bar, even
  with only one tab, so a lone pane can be dragged by its tab to
  another window. Click a tab to show it. Middle-click it, or click its ×, to
  close it. Ctrl+PageDown and Ctrl+PageUp go to the next and the
  previous tab, and Ctrl+Shift+PageDown and Ctrl+Shift+PageUp move the
  tab in front. Drag a tab along the bar to move it. Drag it onto
  another kakel window's bar to move it there, splits and all. Drop it
  on a pane to join that pane in a split, on the side it is nearest.
  Let it go outside every window and it opens a window of its own.
  In a split each pane has a line naming it: drag the line to dock the
  pane beside another, onto a tab of its own, or out into a window of
  its own.
- **In the tray, and one kakel at a time.** kakel shows an icon in the
  system tray, whose menu lists this computer and every saved server,
  each with Terminal, Files and the rest, and Machines, New Window,
  Secrets and Quit kakel. They work with no window open. Closing the
  last window leaves kakel running there; Exit, or Quit kakel, ends it.
  Starting kakel again, from a shortcut or a shell, hands its command
  line to the kakel running, which opens a window for it in the folder
  it was started from. `KAKEL_ALONE=1` starts one of its own. Settings ›
  General › Show kakel in the tray turns the tray off, and with it kakel ends with its last
  window, as before.
- **A launcher from any program.** Shift+Win+K on Windows, and
  Ctrl+Alt+K on Linux under X11, opens a small window over everything
  that finds a machine by its name as you type, and once you type, the
  shells here by name (`cmd`, `wsl`, PowerShell), files on any machine
  or in a WSL distribution, your saved commands, and the Machines,
  Secrets and a new window. Enter on a machine opens what you
  opened there last since kakel started, a terminal at first, in the
  window you last worked in; Tab lists the rest, as Files and, while
  connected, the Connection Log, and Escape goes back and then closes
  it. Files from the launcher or the tray open in a new tab of that
  window; Settings › General › Files open in can split them beside its
  file manager instead, or give them a window of their own.
  Settings › General › Launcher key sets another key with Ctrl, Alt or Win in
  it, or none; kakel says so if another program has it. Open Launcher,
  on the Machines menu and the tray's, opens it anywhere, and so does
  `kakel -launcher`, for a key bound in the desktop's own settings
  where kakel can take none, as under Wayland.
- **Tool windows.** View › Open Machines Window and Secrets › Open
  Secrets Window give those panes a window of their own. Tab › Tab to
  New Window does it for the tab in front. In a window that holds only
  the Machines pane or the secrets, a pane you open opens in the window
  you last worked in, and that window comes to the front. A pane you ask
  for again is shown where it is. A secret you type goes to the terminal
  you last used, in whichever window.
- **Splits that ask in place.** Split Right and Split Down split at
  once, and the new half offers what goes there: `+ Terminal`, one
  button for each shell here and each machine, `+ Command…`, and small
  live images of the other panes, to move one in. Escape gives the
  half back.
- **Typing in every pane of a split at once.** Pane › Type in All
  Panes sends what is typed to every terminal in the split on screen,
  each showing its cursor, for the same command on several servers. A
  chip in the title bar says so while it is on, and its × turns it
  off.
- **A file manager in a pane.** The file manager is a pane like a
  terminal: split it beside a terminal, put it in a tab, dock it, or drag
  it to another window, and it keeps its folder, its history and what is
  selected. It takes kakel's theme. Open one from Files on a machine's
  card, beside the file manager in front, or with New File Manager Window
  in a window of its own; a folder opened from Windows opens in one too.
  Its places are the machines: this computer's folders, and each saved
  server under Machines, connected to when you go there. Its menus sit
  behind the button left of Back. Copy, cut and paste work between
  folders and between machines, as kakel's background copies, and a drag
  does too, from Explorer or another program as well. Files on a machine
  with exactly one saved folder open at that folder; with none or
  several, at home. Its dialogs, such as a rename or a name that clashes,
  cover the pane alone, so the terminal beside it keeps working. Its own
  keys work while it has the keyboard, and `Ctrl+Shift` keys stay
  kakel's. A pane narrower than 860 pixels folds its preview away.
- **A viewer for a file, without a shell.** Space or View in the file
  manager, or Ctrl+click on a path in a terminal, shows a file over the
  window, on this machine or on a server: code coloured for some forty
  kinds of file, Markdown rendered with its source a click away, and
  anything else as its bytes in hex. It reads the first 4 MB of a file,
  and only a file: never a device or a pipe. A Markdown document loads
  nothing it points at, and its links open in the browser only when
  they are web addresses. The file manager's preview colours code and
  renders Markdown the same way.
- **A reader for a terminal's scrollback**, from Find in Scrollback,
  that works the way `less` does: a page at a time, "/" to search, "n"
  and "N" for the next match and the one before, ":" to go to a line,
  and Ctrl+H for a hex dump.
- **A strip beside the file**, where a code editor puts its minimap and
  doing the same job: the shape of the whole file at once, drawn in
  pixels, the pane's place in it as a box, and a click or a drag to go
  there. A log's strip is wider and marks how bad each part got, so one
  error in a thousand quiet lines is found by looking rather than by
  scrolling. `Ctrl+M` turns it off.
- **A log of JSON lines read as a log.** A file whose lines are JSON
  objects is laid out in columns -- the time, the level, the message,
  and the rest of the fields after it -- with the level coloured for
  what it means. It turns itself on for a file that looks like one, and
  `Ctrl+J` puts the JSON back. Every logger spells the fields
  differently, so `time`, `ts`, `@timestamp`, `level`, `severity`,
  `msg` and `message` are all read.
- **Links and file paths in the output.** Ctrl and a click follows a
  link a program declared with OSC 8, an address written out in the
  text, or a file the output named. Holding ctrl marks what is under
  the pointer and writes where it goes along the bottom row, because a
  program can put any address under any words. A file opens in the
  viewer and a directory in the browser, at the line a compiler named
  when it named one. A path with a space in it counts when it is in
  quotes, as in "My Notes/todo.md", with a line named inside the quotes,
  after them, or the way Python names one: `"my file.py", line 12`. A path is checked against the disk
  before it counts as a link, so a run of characters naming nothing is just
  text. It works on a server too: the machine at the far end is asked
  over the connection the window already has, and what it says is kept,
  so a path lights up a moment after the pointer reaches it. A relative
  name needs the shell to say where it is, which kakel sets up
  itself; see **Shell integration** below.
- **The files inside WSL.** Every distribution installed is a line on
  the plus for this machine, and the browser reads it like any other
  directory: Windows serves them on a share, so nothing of kakel's
  own is needed. A file dropped on a WSL pane lands in the directory
  that shell is in, on the same share.
- **An image a program put in its output.** OSC 1337, the sequence
  iTerm2 made and the terminals after it copied. The pane holds the
  image on the line it landed on and it scrolls with the text, on a
  layer of its own. It travels to a window watching the pane: a screen
  is sent as the escape sequences that draw it, so the images go the
  same way. Only an inline image is taken -- the same sequence asks a
  terminal to save a file, which a pane should not be able to make this
  window do. An inline image that can't be shown leaves a line saying
  why, such as "[image not shown: it is over 16 MB]".
- **Files dragged into a pane.** They land in the directory the shell
  said it was in, and nothing is typed: the file is already where the
  program is looking. The window says when it has arrived. A pane on a
  server has the file copied there first, with a row saying how far it
  has got. A shell that has not said where it is leaves nowhere to put
  the file, and then the path is typed instead.
- **File work in the background.** Copying, moving and deleting, on one
  machine or between two, with how far along it is and a way to stop it.
  A name that is already there is asked about — replace, skip, rename, or
  stop — and never decided alone. A file is written beside its name and
  moved onto it at the end, so what is at that name is either the file
  that was there or the whole of the new one, never half of either. Every
  failure stops the job and says why: half a directory that says it
  worked is worse than one that stopped.
- **Tunnels you can find again and look inside.** Clicking a tunnel's
  row opens a pane for it: what it has been doing, a way to watch what
  goes through it, and the button that closes it. Watching is off
  until asked for, because a tunnel carries whatever it carries. A
  tunnel can be kept the way a command can, and each one kept is a
  line on the palette.
- **Tunnels.** A port here that stands for a service over there, a port
  over there that stands for one here, or a SOCKS5 proxy that reaches
  whatever it is asked for as the far machine sees it. A tunnel with no
  address of its own listens on that machine only, and one that would
  let the rest of the network through asks before it opens — as does
  every remote forward, because where the far machine really binds it is
  the far machine's decision. The panel shows what each is carrying: how
  many streams, how fast, and how many failed.
- **A Machines pane.** `Ctrl+Shift+L` opens it in a tab of its own, or
  goes to it where it is; drag its tab out to give it a window of its
  own. It lists every terminal, file manager pane, tunnel and transfer, in every
  window, under the machine it is on with this one at the top. A click
  on a row in another window brings that window to the front. Every
  saved server is on it whether or not anything is connected, and every
  machine carries a plus that drops a menu of what can be opened there.
  Quick Connect and Add Server are along its top. A row's hand-drawn kind icon is coloured for what it is
  doing — green for open, brightening and dimming while bytes are going
  past, grey once it has finished — and a machine's own heading carries a
  dot in the same colours, as does a row in a pane too narrow to draw an
  icon. The row of the pane last worked in is lit. A server not saved
  is connected to with Quick Connect, and listed, marked "quick", while
  anything is open on it. Nothing polls: the row is worked out afresh
  each frame from when the last byte went by, so an idle list redraws
  nothing at all. `Ctrl+Shift+B` opens and closes it.
- **Machines are saved.** A machine you add gets a line on the Machines
  menu and an entry in the palette, kept in a JSON file under the OS
  configuration directory. It holds no secret and never will. A list
  that cannot be read is reported and is never written over, because a
  file nobody could parse is still somebody's list of servers.
- **Secrets are asked for where you work.** A key passphrase, an account
  password, the secrets' passphrase and a one-time code each open in a
  small window of their own, over the other windows and with the
  keyboard in the field, so one asked for from a file manager pane is
  typed right there. Enter answers, and Escape or closing
  the window cancels. Several wait their turn, one window at a time. A
  passphrase that does not open the key is asked for again, with no
  limit, and the window says the last one did not work; Cancel stops
  the asking. An unlocked
  key is kept in memory until the window closes or Lock SSH Keys is
  chosen, and never written anywhere, so the second connection to a
  machine asks nothing.
- **Unknown host keys are shown, not assumed.** A host that is not in
  `known_hosts` gets a dialog with its fingerprint, and only an explicit
  yes records it. A key that does not match one already recorded is
  refused with no button to press.
- **The SSH agent is carried only where you say.** "Forward this
  machine's SSH agent to it" in the server dialog lets that machine
  reach the agent running
  here, so a jump onward from it signs with the keys held here and no
  key is copied over. It is off until you turn it on, per machine, and
  while it is on anyone who is root on that machine can sign with those
  keys for as long as the connection is up. A machine that will not
  carry the agent opens no pane, rather than opening one that quietly
  has no keys.
- **Drawn by gunim.** The window is drawn on the GPU by gunim, a
  pure-Go GUI framework by the same author. It loads OpenGL at run time,
  and presents through DXGI on Windows.
- **A theme editor.** Edit Theme, in Settings › Appearance or the
  palette, opens gunim's theme editor in a tab, on the theme the window
  is drawn in. The cursor comes first: it glides along a line, or jumps.
  Then the motion, the accent and other colours, and how round things
  are; All values lists every value gunim and kakel theme, to search.
  A change shows as it is made, and is kept in `themes.json` under the
  theme's name, built-in themes too.
- **The window opens where it was.** Its place and size, and whether it
  was maximized, are kept from the last run. A window whose screen has
  gone opens on one that is there.
- **Damage tracking.** Writing a cell that already holds the same
  content does not dirty its row, and only the rows that changed are
  passed on to be drawn.
- **Wide characters and combining marks.** CJK and emoji take two
  columns; a base character and its marks share one cell. Emoji are
  drawn from the system's emoji font, in colour.
- **Box drawing that joins up.** The box and block characters are drawn
  in code at the exact cell size, so framed TUIs have unbroken lines.
- **A real key pipeline.** Press, release and OS repeat with modifiers,
  correlated with the text they produced, encoded to the bytes a program
  expects — including application cursor mode, which vim and readline
  need.
- **Mouse, selection and clipboard.** Programs that ask for the mouse
  get it; hold Shift to select text anyway. Drag to select, Alt+drag for
  a rectangle. A program may copy text with OSC 52, as an editor over
  SSH does, up to 4 MB; asking to read the clipboard is never
  answered.
- **A window log.** Help → Window Log shows what the window did: when
  it started, each pane opened and closed, how a shell ended,
  connections made and lost, serving turned on and off, and every
  failure a pop-up told of. It keeps the last 2,000 lines, until the
  window closes. Find in Scrollback searches it, and a connection log,
  as it does a terminal.

![selecting text with the mouse](docs/selection.png)

![vim running on the alternate screen](docs/vim.png)

## Telling a program which terminal this is

`TERM` names a kind of terminal and every terminal borrows the same few
names, so a program reading it learns nothing about this one. Kakel
says which it is in two ways:

- **`TERM_PROGRAM` and `TERM_PROGRAM_VERSION`** in every pane it starts.
  A pane in a WSL distribution gets them too: a Windows variable does
  not cross unless `WSLENV` names it, and kakel adds the two names to
  whatever is already carried.
- **XTVERSION**, `CSI > q`, answered with the same name and version.
  That is the way of asking that survives ssh and tmux, where an
  environment variable does not.

Both say `kakel`, which is true and which no program has heard of
yet. TERM_PROGRAM in Settings › Terminal changes the
name to a terminal a program does know, which is how to make one show
images before it has heard of this one. It may then send the rest of
that terminal's sequences, and whatever kakel does not read lands on
the screen as text. That is the trade, and it is why the honest name is
the default.

## Shell integration

A shell is a separate program, and kakel only sees the bytes it
prints. So it cannot know which directory the shell is in, or where one
command's output ends and the next begins, unless the shell says so. The
shell says so by printing escape sequences nobody sees: OSC 7 or OSC 9;9
for the directory, OSC 133 around each command.

Kakel sets this up itself. As a shell starts it types one line in,
the way you would type it, and then clears the pane. There is nothing to
install and no profile to edit.

- **On this machine it is on**, and "Shell Setup" on the Machine menu
  turns it off. It is invisible: kakel
  builds the line for whichever shell the pane runs. A profile gets it
  only when it runs one of the shells in the table below.
- **On a server it is off**, and "Teach its shell to say what it is
  doing" in the server dialog turns it on. It is off because the line goes into whatever
  login shell that account has. bash and zsh understand it; fish, a
  device CLI or a menu would answer with an error.
- **Another kakel is never set up from here.** The window over there
  starts the shell and applies its own answer.

A program can say things of its own through the same channel. A
message (OSC 9) goes on the pane's row in the Machines pane, into the
window's log, and up as a Windows notification, so one that arrives
while you are looking elsewhere is still seen. How far along it is
(OSC 9;4) goes on the row.

Not a dialog: a dialog takes the keyboard, and anything that can write
to a pane could send one of these one after another. For the same
reason there is one pop-up every two seconds at most, and the ones
left out are still on the row and in the log. The notification is a
balloon in the notification area, which Windows 10 and 11 turn into a
toast and a line in the action centre. The icon appears the first time
a program asks for one and goes when the window closes.

A program may also ask what colour something is drawn in: the text
(OSC 10), the background (OSC 11), the cursor (OSC 12, the text's
colour, which it is drawn in) or one of the 256 palette entries
(OSC 4). All are answered, which is how a program works out whether it
is on a dark theme and picks a colour that will show against it.
Setting a colour is not: the colours are the window's theme, and a
pane left unlike every other one would have nothing to put it back.

What each shell is told:

| Shell | Directory | Command marks |
|---|---|---|
| PowerShell, pwsh | OSC 7, wrapping the prompt already there | yes, with PSReadLine |
| bash, zsh, WSL, Git Bash, a server's login shell | OSC 7 | yes |
| Command Prompt | OSC 9;9 | no |
| A profile running any other program, such as nushell or ssh | none | no |

The Command Prompt builds its prompt out of what `cmd.exe` substitutes,
and none of those pieces makes a URL, so it sends the plain path that
Windows Terminal uses. It has no hook for a command starting or ending.
