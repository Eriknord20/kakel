# Changelog

What changed in each release, for somebody deciding whether to take it.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the numbering follows [semantic versioning](https://semver.org/spec/v2.0.0.html).

While the major version is 0 the shape is still moving: a minor bump may
change how something behaves.

## Unreleased

### Added

**Profiles, for shells kakel does not find.** Settings › Terminal ›
Profiles adds a shell by its command line, such as MSYS2 with
`C:\msys64\usr\bin\bash.exe --login -i`, nushell or `ssh host`. A
profile can start in a folder of its own and set variables, which may
use ones set already, as `PATH=C:\tools;%PATH%` does. It is
listed with Command Prompt, PowerShell, Git Bash and the WSL
distributions everywhere they are: the palette, the split chooser, the
launcher, the tray and the Shell setting. kakel types its setup line
into bash, zsh, sh, ksh, dash, PowerShell, Command Prompt and WSL,
unless the profile gives it a command to run, as `bash -c` does;
another program starts as it is. A saved server has profiles of its
own, such as `tmux new -A -s main`, and its dialog can make one what
its New Terminal opens. These need a POSIX server: variables go through
`env`, the folder through `cd`, and `.profile` is read only when the
command asks for a login shell, as `bash -l` does.

### Fixed

**The title bar is one colour again behind the tabs.** Since the tabs
moved to gunim's shared tab bar in v0.12.1, the room around the tabs
showed the terminal's darker background instead of the title bar's.

## v0.12.2-beta.4

### Added

**Disk usage reads a whole drive.** Right-click a drive, a user folder
or a favourite in the Files sidebar and pick Disk usage. Before, Disk
usage read only the folder showing or a folder inside it, so the root
of a drive was out of reach. A place on another machine than the
window's has no Disk usage line. Open a window there to read it. A
place that cannot be read shows the line dimmed. This comes with gunim
master 0f26d56.

## v0.12.2-beta.3

### Added

**Git Bash runs in a pane.** When Git for Windows is installed, Git
Bash is one of the shells under New Terminals, next to PowerShell and
the WSL distributions. Before, the way to run it was `git-bash.exe`,
which opens a window of its own. A Git Bash pane says which folder it
is in, so a new terminal from it starts there, and a file dropped on it
lands in that folder.

## v0.12.2-beta.2

### Fixed

**Commands with spaces in their path run.** Run Command split the line
at every space, even inside quotes, so a command such as
`"C:\Program Files\Git\git-bash.exe"` could not start. Double quotes
now keep a word whole, and so do single quotes at the start of a word.
Saved commands, pane titles and commands run through another kakel
window keep the quotes too.

**A SOCKS tunnel's refusal reaches the program on Windows.** When a
dynamic tunnel turned down a request, such as one with a bad host name,
Windows could reset the connection before the program read why. kakel
now reads the rest of the request before it closes the connection.

**Servers behind a window you let go of are not marked as lost.** A
server reached through another kakel window went with it when you
disconnected that window. Sometimes it was marked as dropped, with a
greyed row and a "Disconnected" notice, as if its connection had
failed. It is now let go of with the window every time.

## v0.12.2-beta.1

### Changed

**The palette starts with your machines, the one used last first.**
Ctrl+Shift+K now lists each machine before the commands. The machine
you last had a pane in front on comes first, then the ones before it,
then the rest. kakel keeps this order while it runs, and starts afresh
when it starts again.

### Removed

**New Terminal and Files lines in the palette.** The palette began with
"New PowerShell" (or the shell in front) and "Files on This computer".
Each machine's line has these as chips, so the separate lines are gone.
Typing "terminal" or "files" finds the machine lines. The File menu and
Ctrl+Shift+T still open a new terminal like the one in front.

## v0.12.1

Everything from the three betas, v0.12.1-beta.1 to v0.12.1-beta.3, for
those coming from v0.12.0, plus the changes made since. Each beta's own
notes are in CHANGELOG.md.

### Added

**Disk usage, to see what takes the room in a folder.** Pick View ›
Disk usage in the file manager, or Disk usage on a folder's menu. It
reads everything under the folder, on this computer or on any machine
the file manager reaches, and shows it three ways: boxes sized by what
each item holds, a tree of the folders with their sizes, and the types
of file. Double-click a box to go into its folder, and Backspace to come
back out. Files that OneDrive or another cloud provider keeps are
counted apart: what is only online, what is on this device and could be
freed, and what is always kept here. The boxes can be sized by what is
on this device or by what is online, and coloured by cloud, to find the
folders that hold the most offline files. Delete moves an item to the
trash, Shift+Delete deletes it for good after asking, and Show in Files
opens a new tab on the item with it selected.

**The file manager's filter can look in the folders under this one.**
Click the button beside the filter field, or pick Go › Filter
subfolders too. What you type then finds matching items anywhere under
the folder, and a Folder column says where each one sits. Right-click
an item and pick Go to its folder to open the folder it is in, with
the item selected. The search stops at 200,000 items or 30 seconds, and
the status bar says when it did.

**A relay, for windows that cannot reach each other.** `kakel -relay`
runs on a server with no screen and joins a served window and a window
connecting to it, over WebSockets on HTTPS. It gets a certificate from
Let's Encrypt with `-domain`, or runs on plain HTTP behind nginx or
Caddy. Its URL holds a secret, and it answers nothing without it. The
Serve dialog can serve through a relay, Connect to Window takes a
relay's URL and a code or the copied link, and a saved kakel window can
be one served through a relay. Kakel reaches a relay again by itself
when the network comes back, and through the proxy `HTTPS_PROXY` names
or, on Windows, the system's proxy; Settings › Sharing › Relays ›
Proxy overrides both. Serving, connecting and saved windows can all go
through a saved server, as servers behind a jump host do, and a saved
server's Through can be a saved kakel window.

**Tunnels with plain ssh.** A served window takes `ssh -N -L`, `-J`
and `-W` with the keys it allows. The user name says where the tunnel
goes out from: `kakel@` for the served machine, or a machine saved
there by name. With Settings › Sharing › Tunnels may connect on, a
saved server is connected to on the way when that needs nothing asked.
`kakel -relay-pipe` is the ProxyCommand that takes ssh through a relay.
"Tunnels with ssh…" explains it, with the commands for this window to
copy.

**Walk into zips in the file manager again.** A zip opens as a folder,
read in place rather than loaded into memory, so a large one on a
server opens as fast as a small one. A zip with a password asks for it.
Copy items out of a zip as out of any folder, to this machine or
another.

### Changed

**The tab bar is gunim's shared one,** the same as Hej DAW!'s. It looks
and works as before: tabs slide, the front tab's fill glides, and cut
titles show whole in a tooltip. A tab you drag now lifts as a card that
trails the pointer and says where it would land ("Dock here"), and a tab
you click comes to the front at once. The bar shows with one tab too, so
you can drag a lone pane by its tab onto another window; the window it
left closes.

**Copies show as cards in the corner of Files.** The progress panel
across the bottom is gone. Each copy, move or delete running has a card
over the bottom right corner, 360 px wide however wide the window. It
leads with how many files are done and the time left, names the source
and destination folders on their own line, and draws a graph of the
speed across how far the copy has got. While many items copy it shows
bytes and items a second; once done it shows both as averages, which
leave out time spent paused or waiting. Pause holds a copy and Resume
carries it on, and the card says when it waits on a Replace question.
The chevron folds a card to one line with a progress ring; a third copy
starting folds the oldest. The progress moves evenly, and the graph and
the words show the same speed. A finished copy shows Undo and a close
button instead of a toast, and a copy that took 15 seconds or more stays
until you close it.

**The palette (Ctrl+Shift+K) gives each machine one line with chips,**
as the launcher does. A machine is found by its name or address, and
what opens on it shows as chips: Terminal (or this computer's shells by
name), Files, Connection Log, and for a saved server Connect, Edit and
Remove. This computer also gets a Settings chip. Tab, Shift+Tab, Right
and Left move along the chips, and the chip last picked on a machine is
lit there next.

**The cursor's glide is a slider, and every animation has a speed.** In
the theme editor, Cursor sets how long the cursor takes to glide, from
Jumps up to a second. Motion › Animations sets how long all animations
take, from off to five times as long. Save applies them to every window.

**Error notices stay until you close them,** and their text can be
selected and copied.

**A finished job in a window behind others rings and sounds at once.**
Its rings and sound used to wait until the window came back into sight.

**A failed clipboard copy says so.** When the clipboard refuses a copy,
kakel shows a failed notice with why, which the Window Log keeps too.
A program copying bytes that are not text with OSC 52 (such as a
keystore sent through base64 once, which Windows refuses because it
holds a NUL) is turned away, instead of saying "Copied" and pasting
nothing; the notice says how to copy a file's base64 instead.

### Fixed

**The pin button is drawn properly on screens with a colour profile.**
On such screens, the pin before the minimize button showed as diagonal
streaks, and other icons and text could be sheared the same way.

**No dark strip beside the menu button.** Once a second tab opened, the
title bar showed a dark gap between the ☰ button and the first tab.

**A terminal tab through a window keeps its machine in the title** when
the tab is short of room, as "Windows PowerShell on XPS25" did not.

**A folder opened from the desktop opens on the screen.** Its window
opened beside the kakel window in front, past the edge of the screen
when that window was near the edge or minimized.

**A popup no longer hangs past the side of its window** when it fits in
the window's width: it slides sideways to stay inside.

**Disk usage scrolls the tree to the right row.** A box clicked deep
down often scrolled the tree to the wrong place; its row now goes to
the middle of the tree. A click on a row in the tree no longer scrolls
it.

**A pane opened through another kakel window keeps its tie to it.**
When the other window named the pane before it arrived here, the name
was lost, and starting the pane again opened a second shell on the
server instead of restarting the one that ended.

## v0.12.1-beta.3

### Changed

**The tab bar shows with one tab too.** A window with a single tab used
to show its title in place of the bar. Now the tab is there, so you can
drag a lone pane by its tab onto another window, to join its tabs or
dock beside one of its panes. The window it left closes.

**Copies show as cards in the corner of Files.** The progress panel
across the bottom is gone. Each copy, move or delete running now has a
card over the bottom right corner, 360 px wide however wide the window.
It shows the speed, the time left, and a graph of the speed across how
far the copy has got. Pause holds a copy and Resume carries it on. The
chevron folds a card to one line with a progress ring; a third copy
starting folds the oldest. The progress moves evenly rather than in
jumps, and the graph and the words show the same speed.

**Disk usage: Show in Files opens a new tab,** on the item's folder with
the item selected, and Disk usage stays as it was.

**Disk usage can delete.** Delete moves the item selected to the trash,
and Shift+Delete deletes it for good after asking. Right-click an item
for Move to trash and Delete permanently. The sizes take it off at once.

### Fixed

**Disk usage scrolls the tree to the right row.** A box clicked deep
down often scrolled the tree to the wrong place; its row now goes to
the middle of the tree, clear of the shading at the top. A click on a
row in the tree no longer scrolls it.

**A pane opened through another kakel window keeps its tie to it.**
When the other window named the pane before it arrived here, the name
was lost. Starting the pane again then opened a second shell on the
server instead of restarting the one that ended.

## v0.12.1-beta.2

### Added

**Disk usage, to see what takes the room in a folder.** Pick View ›
Disk usage in the file manager, or Disk usage on a folder's menu. It
reads everything under the folder, on this computer or on any machine
the file manager reaches, and shows it three ways: boxes sized by what
each item holds, a tree of the folders with their sizes, and the types
of file. Double-click a box to go into its folder, and Backspace to come
back out. Files that OneDrive or another cloud provider keeps are
counted apart: what is only online, what is on this device and could be
freed, and what is always kept here. The boxes can be sized by what is
on this device or by what is online, and coloured by cloud, to find the
folders that hold the most offline files. Show in Files takes you to an
item in the file list. On Windows, right-click an item in a OneDrive
folder for Free up space or Always keep on this device, as in Explorer.
The same works for a Windows machine's files through a kakel window
there, once that machine runs this kakel too.

**The file manager's filter can look in the folders under this one.**
Click the button beside the filter field, or pick Go › Filter
subfolders too. What you type then finds matching items anywhere under
the folder, and a Folder column says where each one sits. Right-click
an item and pick Go to its folder to open the folder it is in, with
the item selected. The search stops at 200,000 items or 30 seconds, and
the status bar says when it did.

**A relay, for windows that cannot reach each other.** `kakel -relay`
runs on a server with no screen and joins a served window and a window
connecting to it, over WebSockets on HTTPS. It gets a certificate from
Let's Encrypt with `-domain`, or runs on plain HTTP behind nginx or
Caddy. Its URL holds a secret, and it answers nothing without it.

**Serve a window through a relay.** The Serve dialog can serve through
a relay as well as on a port, or instead. Kakel makes a code to type at
the other end and keeps it from one run to the next. Copy Link and New
Code are in the served dialog, which says whether the relay is reached.
Kakel reaches the relay again by itself when the network comes back.

**Connect to a window through a relay.** Connect to Window takes a
relay's URL and a code, or the copied link pasted whole. A kakel window
in the server list can be one served through a relay, with its code.

**Relays through HTTP proxies.** Kakel reaches a relay through the
proxy `HTTPS_PROXY` names or, on Windows, the system's proxy. Settings ›
Sharing › Relays › Proxy overrides both, or says none.

**Windows and relays through a saved server.** Serving through a relay,
connecting to a window, and a saved kakel window can go through a saved
server, as servers behind a jump host do.

**Servers through a kakel window.** A saved server's Through can be a
saved kakel window, served directly or through a relay. Kakel connects
to the window first, and the window opens the stream to the server from
its own machine.

**Tunnels with plain ssh.** A served window takes `ssh -N -L`, `-J`
and `-W` with the keys it allows. The user name says where the tunnel
goes out from: `kakel@` for the served machine, or a machine saved
there by name. With Settings › Sharing › Tunnels may connect on, a
saved server is connected to on the way when that needs nothing asked;
off, as at first, only the servers connected there are reached.
`kakel -relay-pipe` is the ProxyCommand that takes ssh through a relay.
"Tunnels with ssh…" explains it, with the commands for this window to
copy.

**Walk into zips in the file manager again.** A zip opens as a folder,
read in place rather than loaded into memory, so a large one on a
server opens as fast as a small one. A zip with a password asks for it.
Copy items out of a zip as out of any folder, to this machine or
another.

### Changed

**The cursor's glide is a slider, and every animation has a speed.** In
the theme editor, Cursor sets how long the cursor takes to glide, from
Jumps up to a second. Motion › Animations sets how long all
animations take, from off to five times as long. Save applies them to
every window.

**Error notices stay until you close them,** and their text can be
selected and copied.

### Fixed

**No dark strip beside the menu button.** Once a second tab opened, the
title bar showed a dark gap between the ☰ button and the first tab.

**A terminal tab through a window keeps its machine in the title** when
the tab is short of room, as "Windows PowerShell on XPS25" did not.

**A folder opened from the desktop opens on the screen.** Its window
opened beside the kakel window in front, past the edge of the screen
when that window was near the edge or minimized.

**A popup no longer hangs past the side of its window** when it fits
in the window's width: it slides sideways to stay inside.

## v0.12.1-beta.1

### Fixed

**The pin button is drawn properly on screens with a colour profile.**
On such screens, the pin before the minimize button showed as diagonal
streaks. Other icons and text could be sheared the same way.

## v0.12.0

Everything from the 23 betas, v0.12.0-beta.1 to v0.12.0-beta.23, for
those coming from v0.11.1. Each beta's own notes are in CHANGELOG.md.

### Added

**The file manager is a pane.** It splits beside a terminal, goes in a
tab, docks, and moves between windows like any pane, and it takes
kakel's theme. Files opens one beside the pane in front. Files in a
New Window, and a folder opened from Windows, open a kakel window
holding one, which keeps its own size and place. Its dialogs cover the
pane alone, so the terminal beside it keeps working.

**One menu bar that follows the pane in front.** While a file manager
is in front, its lines join kakel's File, Edit and View menus, and a Go
menu comes after View. Edit's Cut, Copy, Paste and Select All act on
the files.

**In the file manager:**
- Open with… on a file's menu, on Windows, as a submenu.
- Extract… unpacks a zip or tar archive (.zip, .tar, .tar.gz, .tgz,
  .tar.bz2, .tbz2) into a new folder beside it, on servers too.
- Create zip… and Paste as zip…, across machines too.
- Zips with a password: it opens ZipCrypto and AES zips, and Create
  zip… and Paste as zip… can encrypt with AES-256. The password
  question offers your saved secrets.
- A viewer: Space, or View, shows a file over the window. Code is in
  its language's colours, Markdown is rendered, and anything else
  shows as hex. A Ctrl+click on a file's path in a terminal opens it
  at the line named.
- Git status: a pane showing a folder in a git repository says the
  branch, how far it is ahead or behind, and how many files changed,
  on a line along its bottom.
- The selection slides to its new place, and stretches with Shift.
- Back selects the folder you came out of.
- Files on a server are titled with the server, as "web: log", and
  the path bar starts with the machine's name.
- Show in folder selects the file, as Firefox asks for a download.

**A paste of several lines opens in an editor first**, coloured for the
pane's shell: bash, PowerShell or the Command Prompt. Lines written for
another shell get marked, with a note on what to write instead.
Settings › Terminal › Show before pasting turns it off.

**All Panes (Ctrl+Shift+A) shows every kakel window over the whole
screen**, as cards with live panes. Pick a pane to go to it, or drag
panes between windows. The launcher offers it too.

**A theme editor.** Edit Theme, in Settings › Appearance or the
palette, edits colours, motion, corners and text size, with a preview.
Nothing changes until you press Save.

**Panes in a split have titles you can drag**, to dock the pane
somewhere else, make it a tab, or give it a window of its own. A middle
click on the title closes it.

**Secrets can lock themselves after a time unused**, from 1 minute to 8
hours, or Never (the default).

**Beta releases.** Settings › General › Beta releases has updates take
betas. `install.ps1` and `install.sh` install the newest beta with
`KAKEL_BETA=1`.

### Changed

- Servers is now called Machines.
- Settings opens as a tab in the window in front.
- Switching tabs glides the highlight and fades in what the tab holds.
- The + on the tab bar opens a tab like the one in front.
- Dragging a tab or pane over another tab brings that tab to the
  front.
- The launcher (Win+Shift+K) and the palette (Ctrl+Shift+K) use the
  same words, list shells by name, and offer saved windows, copies and
  tunnels. A machine's choices are chips on its line.
- Files from the launcher or the tray open in a new tab. Settings ›
  General › Launcher › Files open in changes it.
- New terminal commands say what they open, such as "New PowerShell 7"
  or "New Terminal on skylake".
- A pane on a server is named after the server.
- A pane in another kakel window shows the window icon and says which
  machine it is on.
- Secrets: Edit shows the password, "For" is now "User name", and
  Type says which terminal it types into.
- A failed copy says in words what failed and why, and offers Retry.
- The Jobs pane lists the newest job first, with times, on redesigned
  cards. Click a folder on a card to open it in Files.
- A repeated job and the file manager ask the same Replace question.
- Remove, not Forget, for saved commands, tunnels, favourites and
  copies.
- Text that does not fit fades out instead of ending in "…". Hover to
  see it whole.
- The file list's Size and Modified columns fit their text.
- Coloured terminal text that contrasts less than 4.5:1 with its
  background is lightened or darkened until it reads.
- Number fields in the theme editor and Settings change as you drag
  them.
- Every notice and error kakel shows is also in the Window Log.

### Fixed

- No more freezes on Intel graphics on Windows (gunim issue #30).
- Ctrl+C stops a program in cmd or PowerShell again after an update.
- A folder opened from Windows comes to the front.
- Question windows keep their size and no longer snap to half the
  screen.
- A docked terminal keeps its text as it slides in.
- The file list stays where you scrolled it, and a long Shift+Down
  keeps its selection.
- The arrow keys move through a tall sidebar, and go on from where a
  deleted file was.
- A long path in the path bar scrolls with the wheel.
- Text keeps its width through a theme switch, and a row's text and
  icon scroll together.

### Removed

- The old file pane, with its key bar, Go To (`Ctrl+Shift+G`) and zip
  files opened as folders. The file manager does what the rest did.
- View in Reader and Follow in Reader. The viewer replaces them.
- The Updates dialog. Settings › General › Updates has the same choice.
- "New Terminal, Default Shell". Type a shell's name to open it.

## v0.12.0-beta.23

### Changed

**Job cards in the Jobs pane are redesigned.** Each card shows:
- A badge with the kind of job (copy, move or delete), coloured by how
  it is going: the accent while it runs, green when done, red when it
  failed, grey when stopped.
- A heading that says what it does to what, such as "Copying 6 items"
  or "Couldn't copy gone.txt", with one line on progress or how it
  ended.
- Where it copies from and to: two boxes, each with the machine, the
  folder's name and its full path. A delete shows the one folder.
- The files it works on, each marked waiting, in progress or done, up
  to 12.
- The progress bar and speed graph only while the job runs.
- Save as a bookmark button, Repeat, and an ✕ that takes a finished
  job off the list.

**Click a folder on a job card to open it in Files**, on its machine,
in a new tab. Tab and Enter work too.

**Text that does not fit fades out** instead of ending in "…", which
often cut off too much. This covers tab titles, the file list and
other tables. Hover to see the whole text.

**A server's tab keeps its folder name.** A Files tab on a server is
titled "Skylake: build". When that does not fit, the tab shows "build"
alone before fading.

**The file list's Size and Modified columns fit their text,** so Name
gets the room they leave. They widen if a wider value scrolls into
view and never narrow while the folder is open. Type keeps its width,
and you can still drag it.

**Secrets can lock themselves after a time unused.** The "Auto-lock:
…" button beside Lock (also Auto-Lock Secrets in the Secrets menu and
the palette) offers Never, 1, 5, 15 or 30 minutes, or 1, 4 or 8 hours.
Never is the default. The time counts from the last time a secret was
read or changed. It is kept inside the sealed secrets, so only someone
who can open them can change it.

### Fixed

- A Files tab on a server showed only "Skylake: " when tabs were
  narrow: the folder name was cut off.
- A job card opened after the job had ended showed an empty progress
  bar.

## v0.12.0-beta.22

### Changed

**The launcher (Win+Shift+K) offers everything saved.** Saved kakel
windows are listed as machines, with a window icon and "kakel window".
Saved copies ("Copy a.txt") and saved tunnels ("Tunnel …, via web") are
there too, and picking one runs it again. The palette (Ctrl+Shift+K)
lists saved kakel windows as well.

**Files from the launcher or the tray open in a new tab**, not split
beside a file manager in the window last worked in. Settings › General
› Launcher › Files open in changes it: New tab, Beside, or New window.

**A terminal on another kakel window says which machine it is on:**
"PowerShell 7 on skylake", in its tab, the window's title and All
Panes.

**Secrets: Edit shows the password.** Edit (was Change) opens with the
saved password or note in, hidden until Show password is ticked. "For"
is now "User name", the account the secret signs in as. A new secret
is named after the server in front. A secret saved at sign-in says
where it signs in by itself.

**Secrets: Type says which terminal it types into,** such as "Type
into PowerShell 7 on skylake", and types into that one only. With no
terminal to type into, it is greyed out.

**One replace question.** A repeated job and the file manager ask the
same way when a file is there already: Replace “name”?, with the sizes
and dates of both, Apply to all, and Skip, Keep Both, Stop and Replace.
Keep Both puts the new one beside under a free name.

**Remove, not Forget.** Saved commands, tunnels, favourites and copies
are removed, as servers, secrets and keys are.

### Fixed

- The title bar's "Agent Share" and "Serving" chips sat on a dark
  patch: the window's colour showed through behind them.
- An SSH agent message named a command that does not exist, "Forget
  unlocked keys". It now names Lock SSH Keys.

## v0.12.0-beta.21

### Changed

**A failed copy says why, and offers Retry.** "The copy stopped" and
the file manager's other failures now say in a sentence which item
failed and why, such as "app.exe wasn't put in place: access was
denied. It may be read-only or open in another program, or, if it is a
program, running." The system's own error follows, small, to copy.
Retry starts the operation again for what is left: the items a copy or
a move didn't bring, or the ones a delete didn't reach.

**The Jobs pane lists the newest job first, and says when each ran:**
"17:02, took 4 s", or "started 17:02" while it runs.

## v0.12.0-beta.20

### Changed

**A new terminal says what it opens.** In the command palette
(Ctrl+Shift+K) and the File menu, "New Terminal" is called for what
Ctrl+Shift+T opens from the pane in front: "New PowerShell 7" or "New
Command Prompt", the shell of the pane in front or else your default,
or "New Terminal on skylake" for a pane on a server. "New File Manager"
is "Files on This computer", or on the server in front, and "New File
Manager Window" is "Files in a New Window". The old words still find
them. "New Terminal, Default Shell" is gone: type a shell's name to
open it.

### Fixed

**The launcher's chips brighten as the light slides onto them.** A
chip's text changed at once as Tab lit it, while the light was still
on its way.

## v0.12.0-beta.19

### Added

**The paste dialog marks lines written for another shell.** Pasting
bash into PowerShell, PowerShell into the Command Prompt, or either
into bash, the lines that won't work there get a red line number, a
wavy line under the part that is wrong, and a short note on what to
write instead, such as "bash: write $env:NAME = 'value'". The dialog
says how many lines look written for another shell, and the marks
follow your edits.

**Settings and About kakel in the launcher.** Win+Shift+K finds them
by name, and by words like "preferences", "version" and "updates".

### Changed

**The launcher and the command palette use the same words.** Both say
"Terminal on X", "Files on X" and a favourite by its name, so "files on
skylake" finds the same thing in either. The palette's old words, such
as "new file manager", still find them. The palette also lists saved
servers you aren't connected to.

**This computer's shells go by their names.** Where kakel knows the
shells here, both boxes list each one by name instead of "Terminal",
the one a new terminal starts first and marked "default".

**A machine's choices are chips on its line in the launcher.** The
machine lit shows what can be opened on it at the end of its line, the
one Enter opens lit. Tab and Shift+Tab, or Right and Left, slide the
light along them; a click opens one. This replaces the second list Tab
used to open.

### Removed

**The Updates dialog.** Settings › General › Updates has the same
choice, beside Beta releases and About kakel.

## v0.12.0-beta.18

### Fixed

**The arrow keys move through the file manager's sidebar.** In a sidebar
too tall for its window, Up and Down scrolled it instead of moving from
place to place. They now move the keyboard on, and scroll only past the
first or the last.

**After a delete, the arrow keys go on from where the file was.** With
the keyboard on a file and the file deleted, the next Down started from
the top of the view. Down now goes to the file that came after the one
deleted, in the order the list is sorted, and Up to the one before. In
the icons, Right goes to the tile that took its place.

## v0.12.0-beta.17

### Changed

**Show in folder selects the file.** When kakel opens folders and a
program asks Windows to show a file in its folder, as Firefox does for a
download, the file is selected. Windows gives kakel only the folder, so
kakel selects the newest file in it from Windows' Recent items, where
Firefox puts each download. `-files` also takes a file now, and opens its
folder with the file selected.

### Fixed

**The file list stays where you scrolled it.** After Page Down, or
scrolling away with the wheel, the list no longer jumps back to the
selected row a moment later.

**A long Shift+Down keeps its selection.** Every couple of seconds the
selection could restart from the row the keyboard was on, dropping the
rows above it.

**The folder's preview stays still.** With nothing selected, the preview
of the folder no longer slides in again each time the folder changes.

**Text keeps its width through a theme switch.** Switching to a theme
with another text size, such as Paper to Phosphor, no longer makes the
text jump narrower as the switch settles. Text at sizes between whole
pixels, such as the file list's, was spaced a little too wide and is now
spaced right.

**A row's text and icon scroll together.** Lists scroll by whole screen
pixels, so a row's text no longer wavers a pixel against its icon while
the list slides.

## v0.12.0-beta.16

### Added

**Zips with a password.** Extract… opens a zip a password protects,
made by kakel, 7-Zip, WinZip or `zip -P`, both the old kind and AES. It
asks for the password before it makes the folder, and again when the
password is wrong. Create zip… and Paste as zip… have a box, Protect
with a password, that encrypts what the zip holds with AES-256. The
password question offers your saved secrets, and can save what you type
in them once it has worked. A secret named like the zip opens it
without asking. Before, such a zip failed with "flate: corrupt input"
or "unsupported compression algorithm".

### Changed

**A file manager window opens where the last one closed, and as big.**
A window holding only file managers, as a folder opened from Windows'
Run box gets, keeps its own size and place, apart from the terminals'
window. The next one opens there, maximized if it was. One opened while
another is still there opens a step down and right of it.

### Fixed

**Long names in the file manager's sidebar no longer run past their
row.** A favourite, place or machine whose name is too wide ends in
"…", and so does its note. Rest the pointer on it to read it whole.

## v0.12.0-beta.15

### Changed

**The file list's selection slides.** In a file manager's details view,
the highlight slides to its new place instead of jumping: for the arrow
keys, `Page Up`, `Page Down`, `Home` and `End`, a click, and a name
typed at the list. With `Shift`, the selection stretches and shrinks,
and `Ctrl+A` stretches it over every row. A move farther than the view
still jumps, and rows picked apart with `Ctrl` and a click do not slide.

**Back comes back to the folder you left.** `Backspace` (or Back) now
selects the folder you came out of, at every level. So you can select a
folder, press `Enter` to look in, press `Backspace`, then `Down` and
`Enter` to look in the next one. Forward does the same.

## v0.12.0-beta.14

### Changed

**The git status is in the file manager, not under terminals.** A file
manager pane showing a folder on this computer that is in a git
repository says the branch, how far it is ahead of or behind its
upstream, and how many files have changed, on its own line along the
bottom. The line under the panes no longer says it for a terminal. It
is read again every 3 seconds, and at once when the pane opens another
folder.

## v0.12.0-beta.13

### Fixed

**No more freezes on Intel graphics.** Two hangs that stopped every
window at once, "Not Responding", are gone (gunim issue #30):

- Intel's Windows graphics driver could deadlock when one window blurred
  while another made a picture. kakel now makes its smaller copies of
  pictures itself, on the graphics card, without the driver call that
  deadlocked.
- kakel ran its threads at raised priority. Under heavy load, that kept
  Windows from running the threads Go needs to hand work between them,
  and everything stood still for 4 to 11 seconds. Threads now run at
  the priority Windows gives them. Frames may drop when other programs
  load the machine heavily, but nothing freezes.

**All Panes puts windows back at their own size.** Closing All Panes,
the cards could land smaller than the windows they stand for. Each
window's place is now measured by All Panes' own window, at the scale
it draws at, not worked out from the monitor's. The windows also fade
back in at their own size, where before they grew from a little
smaller. If a card still lands at the wrong size, start kakel with
`KAKEL_DEBUG_OVERVIEW=1` and the Window Log says where each window was
measured.

## v0.12.0-beta.12

### Added

**A paste is coloured for its shell.** The editor a multi-line paste
opens in now colours the code the way the pane's shell reads it: bash
and other POSIX shells, PowerShell, or the Command Prompt. Variables,
strings, keywords, commands and comments each get a colour from the
theme's terminal palette. The text stays plain while a program is
running in the shell.

**Extract archives in the file manager.** Right-click a zip or a tar
archive (.zip, .tar, .tar.gz, .tgz, .tar.bz2, .tbz2) and choose
Extract… to unpack it into a new folder beside it. It asks for the
folder's name first. Undo moves the folder to the trash. It works on
servers too.

**Git status on the status line.** In a terminal on this computer whose
shell is in a git repository, the line under the panes says the branch,
how far it is ahead of or behind its upstream, and how many files have
changed. The branch is read from the repository's own files. `git
status` runs only where the repository's settings name no program for
it to run (a filter or a file monitor), so opening a folder never runs
code from it.

### Changed

**A new viewer for files.** Space, or View on a file's menu in the file
manager, shows the file over the window: code in its language's
colours, Markdown rendered (with its source a click away), and anything
else as its bytes in hex. A Ctrl+click on a file's path in a terminal
opens the same viewer, at the line named. It reads at most 4 MB, and
only from a real file. A link in a Markdown document opens only when it
is a web address. View in Reader is gone from the file manager's menu.

**A pane on a server is named after the server.** A new terminal on
"Mikronika" is called Mikronika, and the next one Mikronika 2, until its
shell gives it a title of its own. It was "Terminal 3", counted across
every pane.

**The preview colours code and renders Markdown.** Its picture of a
file without a thumbnail is now the file's own icon, as the icon view
shows it, not a coloured square.

**All Panes clears the screen.** Ctrl+Shift+A fades kakel's windows out
as their cards come out of them, and fades them back in as All Panes
goes. A click outside All Panes, as on another monitor, closes it, with
the same animation.

**Tabs open under a drag.** Drag a tab, or a pane by its title, and
hold it over another tab: that tab comes to the front, so you can dock
the pane beside one there. Let it go on the tab bar instead, and it is
a tab of its own again, in front, where you dropped it.

**A middle click on a pane's title closes it**, as it closes a tab.

**Every notice is in the Window Log.** Each toast and pop-up kakel
shows is written to the Window Log too, so you can read it after it has
gone. Before, only failures were.

### Removed

**Follow in Reader.** The file manager no longer tails a file, and a
scrollback reader no longer follows its pane; Ctrl+F there does nothing.

## v0.12.0-beta.11

### Added

**Create zip and Paste as zip in the file manager.** Right-click items
and choose Create zip… to put them in a zip beside them. Or copy items,
go to another folder, and choose Paste as zip… there. That works across
machines too: copy on this computer, and paste as a zip on a server.
Each asks for the zip's name first. It suggests the item's name, or the
source folder's name for several items.

**Files on a server say which server.** A Files pane on a server is
titled with the server's name and the folder, as "web: log", in its tab
and in the window's title. The bar of folders above the files starts
with the machine's name too. Click it to go to your home folder there.

### Fixed

**A long path scrolls.** When the folders of a path don't fit in the bar
above the files, the mouse wheel or a touchpad scrolls them. They fade
at an edge where more are hidden.

**File icons stay after a Files pane moves.** A Files pane moved to
another window lost the icons of its folder until you changed folder.

**Every failure you see is in the Window Log.** A copy or paste in the
file manager that failed showed its error in a dialog, and the Window
Log had nothing about it. Now every error kakel shows goes there too.
That covers the file manager's dialogs, folders it can't list, and
previews that fail. It also covers a reader that can't read or save
its file, and why a remote window refused or ended a pane. And it
covers each failed step while connecting, a lost connection's reason,
a shortcuts file that names unknown commands, and the update window's
failures. A few failures that showed nothing at all now say so: a link
that didn't open, and settings that couldn't be saved.

**A failure names its file once.** "open D:\x: open D:\x: denied" now
reads "open D:\x: denied".

## v0.12.0-beta.10

### Added

**Panes in a split have titles you can drag.** Each pane in a split
has a line along its top naming it, and the one with the keyboard is
marked. Drag the line to dock the pane on another side of another pane,
onto the tab bar as a tab of its own, or outside the window into a
window of its own. With one tab, let it go above the panes to give it a
tab. A pane dragged from another window docks beside the pane it is let
go over, as a tab does.

### Changed

**The theme editor changes nothing until you press Save.** Your edits
show in its preview only. Save keeps them and restyles every window.
Discard takes them back to what was saved. Closing the editor without
saving drops them.

### Fixed

**Coloured text stays readable on coloured backgrounds.** Some programs
colour text too close to its background in kakel's themes: `ls` shows a
folder anyone may write to as blue on green, and PowerShell shows every
folder on blue. Text that contrasts less than 4.5:1 with its background
is now lightened or darkened until it reads. Text that reads already,
box lines, blocks and Powerline symbols keep their colours.

**A docked terminal keeps its text as it slides in.** While a pane slid
in beside another, both terminals were resized on every frame, so the
program in them wrapped or cut its output to the narrow sizes on the
way. Each pane now keeps its size during the slide and is resized once
at the end.

**A tab docks beside a Files pane.** Dragging a tab to the side of a
Files pane lit up where it would go, but letting go did nothing.

**All Panes fades the screen in.** `Ctrl+Shift+A` turned the whole
screen dark in a flash before the cards came in. Now the screen
darkens smoothly behind them.

**The Ctrl+Tab list closes on Settings.** Walking to Settings or the
theme editor with `Ctrl+Tab` left the list of panes on screen after
`Ctrl` was let go.

## v0.12.0-beta.9

### Added

**A paste of several lines opens in an editor first.** Text of more
than one line, or over 5 KB, shows in a dialog before it reaches the
pane, with Paste and Cancel. Change it there: `Enter` pastes,
`Shift+Enter` starts a new line, `Escape` cancels. Where the program
treats each line break as `Enter`, the dialog says so. Settings ›
Terminal › Show before pasting turns it off.

### Changed

**Number fields drag.** The theme editor's numbers and Settings' font
size change as you drag them up or down: finer with `Shift`, faster
with `Ctrl`. The serving port stays a field you type in.

### Fixed

**Text no longer flickers after saving a theme's edits.** The text in
Settings dipped toward black and back on every update after the theme
editor's changes were saved.

## v0.12.0-beta.8

### Changed

**All Panes lets the screen show through**, darkened, behind the
cards, where the system lets a window be see-through, as Windows does.
Its window has no line round it.

### Fixed

**Terminals in All Panes stay live.** They showed what their windows
last drew, and the windows draw nothing while All Panes covers them;
now All Panes reads each terminal's screen itself, cursor and all.

## v0.12.0-beta.7

### Changed

**All Panes shows every window, over the whole screen.** `Ctrl+Shift+A`
shrinks each kakel window from where it stands into a card, holding its
tabs with their splits as they stand, every pane live. Pick a pane and
it grows back into its window, which comes to the front on that tab;
`Escape` puts every window back. Drag a pane onto a pane to join it in
a split, onto a window's card to move it there on a tab of its own, or
onto empty room to give it a window of its own there. The launcher
offers it as All Panes, so it opens from any program.

**The theme editor is laid out like Settings.** Its values sit in
cards and keep the theme as it was, while a preview beside them shows
the changes and plays each motion. A motion is Instant, Snappy, Gentle,
Bouncy or Custom, with Speed and Bounce, and only a changed value shows
a mark and a reset.

## v0.12.0-beta.6

### Added

**A theme editor.** Edit Theme, in Settings › Appearance or the
palette, opens it in a tab, on the theme the window is drawn in. The
cursor comes first: it glides along a line, or jumps. Then the motion,
the accent and other colours, the corners and the text size; All
values lists every value of the theme, to search. A change shows as it
is made, and is kept in `themes.json` under the theme's name, built-in
themes too.

## v0.12.0-beta.5

### Changed

**Open with is a submenu** in a file's right-click menu. It opens
beside the menu as the pointer rests on it, or with the Right arrow,
and lists the apps once Windows has found them.

**New Terminal and New File Manager come first in the palette**, said
alike. New File Manager was Files Here, and Files in a New Window is
New File Manager Window, further down. The File menu and a machine's
menu use the same words. The old words still find them.

**A drag scrolls a list sooner near its edge.** It starts 48 px from
the edge, at once, and speeds up toward it. A dragged file keeps the
list scrolling just past its edge.

### Fixed

**Ctrl+C stops a program in cmd or PowerShell again.** After an update,
kakel started again ignoring Ctrl+C, and so did every shell in it.

**A file's right-click menu shows all its lines.** It was cut at 480
px and scrolled; now it is as tall as the screen allows.

**F5 refreshes the file manager in front**, also after a click on its
tab or the sidebar. Ctrl+R does too.

## v0.12.0-beta.4

### Changed

**A pane in another kakel window shows that it is.** Its tab has the
window icon in place of the terminal icon, so it is not taken for a
shell on this computer.

**Focus rings fade in place**, and the echo after a click sends one
soft wave. An update starts with a glow round the icon.

### Fixed

**A pane in another kakel window has that window's name for it.** A
Command Prompt there was called `C:\WINDOWS\system32\cmd.exe` here; now
it is called Command Prompt, as it is there.

**The Serving chip sits on the line of the tab titles**, where it sat
about 3 pixels above them.

**The installer's last lines stay clear of its buttons.** With all of
kakel's offers, "For you alone…" and "Or run it without installing"
ran into the Update button. The icon now shrinks to make room.

**Updating over a copy that never recorded its version** said "From
to 0.12.0-beta.4". It now says "To 0.12.0-beta.4, in place of the copy
installed".

## v0.12.0-beta.3

### Changed

**Servers is now called Machines**: the pane, the menu, the tray's
line, the launcher's line and the file manager's group of places. It
lists this computer and WSL as well as servers, so the old name fit
badly.

**Settings opens as a tab** in the window in front, as Machines does,
where it opened in a window of its own. Asked for again, it goes to
the tab where it is.

**Switching tabs shows in motion.** The front tab's highlight glides to
the tab picked, and what it holds fades in from that side, in 150 ms.
What you type goes to the new tab at once.

### Fixed

**A question window keeps its size.** Connect to Server, a password
ask and the other question windows could not be sized or maximized,
yet Windows still snapped them to half the screen or maximized them,
from a drag to the screen's edge or the Windows key and an arrow. Now
they stay as they open.

## v0.12.0-beta.2

### Changed

**One menu bar that follows the pane in front.** The file manager's
own menu button is gone: while a file manager is in front, its lines
join kakel's File, Edit and View menus, a Go menu comes after View, and
the lines for a terminal alone leave. Edit's Cut, Copy, Paste and
Select All act on the files, and `Ctrl+Shift+X` cuts.

**The + on the tab bar opens a tab like the one in front**: a file
manager at the same folder, or a terminal. A right click offers either.

### Fixed

**A folder opened from Windows comes to the front**, as Win+R and
"Documents" with kakel running ask: the kakel started for it lets the
one running bring its window forward, where Windows left it behind.

### Added

**Open with…** on a file's menu in the file manager, on Windows: the
programs Windows offers for the file, and Choose another app…, which
shows Windows' own dialog. A file on a server is fetched first.

## v0.12.0-beta.1

A beta: the file manager as a pane, for those who take betas to try
before it is a release.

### Changed

**The file manager is a pane.** It splits beside a terminal, goes in a
tab, docks, and moves between windows like any pane, and it takes
kakel's theme. Its menus are behind a button left of Back. Its dialogs
cover the pane alone, so the terminal beside it keeps working.

**It replaces the old file pane and the file manager's own windows.**
Files opens a file manager pane, beside the one in front. Files in a
New Window, and a folder opened from Windows, open a kakel window
holding one. The setting that chose between a window and a pane is
gone.

### Added

**Beta releases.** Settings › General › Beta releases has updates take
betas too. A beta takes the next beta by itself, until told otherwise.
`install.ps1` and `install.sh` install the newest beta with
`KAKEL_BETA=1`.

**View in Reader and Follow in Reader** on a file's menu in the file
manager open it in kakel's reader beside the pane, as F3 and F4 did in
the old file pane.

### Removed

**The old file pane, with its key bar, Go To (`Ctrl+Shift+G`) and zip
files opened as folders.** The file manager does what the rest did.

## v0.11.1

### Fixed

**Memory no longer grows as windows open and close.** Every closed
window stayed in memory with all it showed: its terminals and their
history, which can be over 100 MB a terminal. A row of a closed pane in
the Servers pane held its terminal too. Both let go now.

**Changing the theme no longer races a terminal being drawn.** The
colours changed under a window that was copying them, which could draw
a frame wrong.

### Added

**`GUNIM_DEBUG_POPUP=1` says how a dropdown's window is sized**, to find
why the Font list in Settings is cut short on Windows.

## v0.11.0

### Fixed

**Show in system file manager opens File Explorer**, also when kakel
opens folders. It opened kakel again, or failed with HRESULT 0x80004004.

**Ctrl+Tab in a window of one pane, as Settings is, no longer stops
every click there.** It left the ring it slides between panes over the
window, which took the pointer, the title bar's included, while the
keys went on working.

**What the file manager shows going wrong is in the Window Log too**,
past the banner it is dismissed from.

**A freeze that could stop every window on Windows.** A window's
OpenGL context was made, used and deleted partly on the thread every
window waits on, while other windows drew; a graphics driver can wait
there for the others. Each window's context is now made, used and
deleted on its own drawing thread.

### Changed

**Less drawing.** Long text, labels and previews draw only the lines in
view, and the GPU gets nothing that can't show. A scrollbar lingers
without drawing, and a transfer's speed graph draws 30 times a second
rather than at every refresh.

## v0.10.0

### Changed

**Settings is a window of its own, in tabs.** File › Settings… (Ctrl+,),
the gear on the Servers page, and the tray all open it. It holds what
the Options and Font menus did, which are gone: General (start with the
computer, the tray, the default file manager, the launcher key, the
title bar, updates), Appearance (the theme, the font and its size),
Terminal (where terminals start, the shell, TERM_PROGRAM),
Notifications (sounds and rings), Sharing (serving, and the keys that
may connect) and Files (where kakel keeps them, and the themes and
shortcuts files). A switch or a choice takes effect at once; a line
typed is kept with the Save beside it. Sharing also says what kakel
does as it starts when the window was served as it last closed: ask,
serve again, or not, which before only "Don't ask again" could set.

**A stress test for the What's New freeze.** `KAKEL_STRESS_WINDOWS=15m`
has kakel open the What's New window, close it a few seconds later, and
go again, for that long. Should the windows freeze, the hang report says
where. Release notes read once are kept 10 minutes, so the windows don't
ask GitHub each time.

**What's New and the installer's windows are smooth again.** Long notes
draw only the lines in view, and the installer's icon comes to rest
after a few seconds, so the window stops drawing when nothing moves.

## v0.9.1

### Changed

**About kakel is a window of its own.** It shows the version, what each
release changed, and Check for Updates. The check runs with the window
open and says how it went there; a newer release turns the window to
it, to update now.

### Known problem

**Opening What's New froze kakel on Windows, and the cause isn't found
yet.** Should kakel's windows stop answering for 10 seconds, kakel now
writes where each part of it was to `gunim-hang-kakel-<pid>-<time>.txt`
in the temporary folder, which says what held them.

## v0.9.0

### Changed

**Every OneDrive state in the file manager, before the name.** The
cloud mark went after the name, so a long name hid it, and only files
kept online had one. Now a mark before the name says each state, as
Explorer does: a cloud for a file kept online only, a green tick for
one on this device, and a green pin for one always kept there. The
icon view shows the same mark over each tile's bottom right corner, the
same size at every zoom. Files
through a kakel window on Windows show them too, once that kakel is
this version.

**Folders open in kakel, if you like, on Windows.** The installer offers
"Open folders with kakel", and Options › Default File Manager turns it
on and off later. On, a folder or a drive opened anywhere, and Win+E,
open kakel's file manager in place of File Explorer. Off, or kakel
uninstalled, File Explorer comes back. `kakel -files <folder>` opens a
folder in the file manager from a shell.

**Windows' own file icons in the file manager.** On Windows, the file
manager shows the icons File Explorer does, in the details and on the
tiles. View › Windows icons switches back to kakel's own.

## v0.8.0

### Changed

**Updates show what they do.** A newer release opens a window with
what changed since your version, and Update Now. The download shows
its progress, and kakel restarts into the new release: the window says
so until the old kakel has closed, and then that kakel is up to date.
An update installed by itself says so on kakel's next start, with
What's New a click away. About kakel › What's New shows every
release's notes.

**Keys that may connect, from the Serve dialog.** The dialog lists the
keys that may take the window over. Add Key… takes a public key pasted,
or one from this machine's `~/.ssh`; Remove Key… takes one off, and
hangs up on the window it let in. A served window takes the change at
once, with no restart.

## v0.7.0

### Changed

**Back and Forward cross servers in the file manager.** Going from this
computer's files to a server's, or from one server to another, no
longer empties a window's history. Back returns to the folder you left,
connecting to its server first when needed.

**The installer waits for kakel to close.** Updating or uninstalling
while kakel runs goes on by itself once it has closed. Close kakel
there asks the running kakel to end, with a ring turning while it
does, and says so if it didn't.

**Signed updates.** Each release now carries `SHA256SUMS.sig`, a
signature of its checksums. An installed kakel puts an update in place
only when the signature matches kakel's own key, so a release that
anyone else put up never runs. Updates come over https only.

**Leaves other programs' files alone.** Installing and uninstalling
replace or take away a link in `~/.local/bin`, or a desktop file of
kakel's name, only when it starts kakel. An older kakel started from an
old shortcut hands over to a newer one installed, instead of copying
itself over it.

## v0.6.0

### Changed

**An installer of its own.** A kakel started from the zip or the
tarball opens an installer before anything else: kakel's icon on a glow
in its colours, the offers of a desktop shortcut, a start with the
computer and automatic updates, and a ring that goes round as it
installs. It can also run kakel as it is, without installing.
Installing an older or the same version says so, and offers to open the
one installed. `kakel -uninstall` and Installed apps open it to ask, and
it can take your settings and saved servers too.

**Installed on Linux in `~/.local/share/kakel`.** The program and its
files go there, with a link at `~/.local/bin/kakel`, so typing `kakel`
in a shell still starts it. A kakel installed in `~/.local/bin` by an
older release moves itself the first time it starts, keeping its start
with the computer and its shortcut on the desktop.

Both come from gunim, whose installer kakel's grew into, and so do the
updates now. They work as they did: Options › Updates… tells you of a
new release, installs it by itself, or does nothing, and what you had
chosen carries over. The installer's Keep kakel up to date sets it:
ticked, new releases install by themselves; unticked, kakel asks first.

### Fixed

**A new window opens in front on Windows.** A kakel started from
Explorer, the Start menu or a shortcut could open behind the window it
was started from, and had to be found on the taskbar.

## v0.5.0

### Added

**Settings, with sounds.** Options › Settings… is a table of what
kakel tells you of: a connection made, one lost, a long command or a
program ending, the bell, and other errors and finished work. Each can
play a sound and send rings out around the window, and buttons, menus
and switches can make quiet sounds of their own. Unless you choose
otherwise, the rings stay as they were and the bell makes a sound,
which otherwise says nothing in the pane you are looking at. The
speakers open only once a sound is turned on, and rest while kakel is
quiet.

**Use the system's title bar.** A box in Settings gives the windows
opened from then on the window manager's own title bar and frame, and
takes kakel's minimize, maximize, close and pin buttons away. File
manager windows follow it too.

**A bell you can see in the pane in front.** A bell rung where you are
looking lights the window's edges amber and lets them fade, and the
window flashes in the taskbar only when it lacks the keyboard.

**Open a server's files with this computer's programs.** Open and Open
With… on a server's file fetch it, and open it here. Edit it and save,
and kakel offers to send it back, in a notice that waits rather than
interrupts, with "Don't ask again".

**Drag a server's files anywhere.** Drag them onto this computer's
places in the sidebar, or out of the window into another program, as
a mail or a chat: kakel fetches them as the drag starts, and the hint
under the pointer says how far it has got, on a bar.

**Saved kakel windows are like servers.** Connect to one from the tray,
the launcher and the file manager, signing in with your secrets, with
no terminal opened you did not ask for.

**A Windows machine reached through kakel is written its own way.**
Paths read C:\Users\…, typed Windows paths are understood and their
letters' case is found, its drives are listed at the top, and its free
space shows.

**The sidebar can be arranged.** Its sections fold and move, and
favourites start with your Desktop, Documents, Downloads and the like,
each with a colour and an icon you can change with Edit Favourite.

**A terminal lights up under files dragged from another program,** and
a file manager folder lights under them before you let go.

### Changed

**Terminals draw far faster.** A full-screen animation, as termflix
draws, sent the graphics card tens of megabytes a frame; it now sends
about half a megabyte, as the cells, their colours and their letters
are drawn in one pass on the GPU. Output is parsed about twice as fast,
and kakel's own animations stay smooth while a terminal floods.

**Local panes on Windows run through OpenConsole.** It is the console
host Windows Terminal ships, and kakel carries it. Windows' own redrew
a full-screen animation on a timer and tore it across; now each frame
comes through whole. kakel uses Windows' own where OpenConsole cannot
be placed.

**Tabs come and go smoothly.** Closing the second tab fades the tab bar
out and the window's title in, and opening one does the reverse.

**One tray icon,** and a file manager window connects to a server
quietly, without opening the Servers pane.

**Serving at start can be decided once.** The question asks whether to
serve the window again, with a box to remember the answer.

**Chips in the title bar** are drawn as pills, in the middle of the bar.

**`go install github.com/marrasen/kakel@latest` works,** with no
replace directive left in kakel's modules.

### Fixed

**A folder copied from Windows to Linux can be emptied again.** Its
copy kept the Windows read-only attribute as mode 0555, so nothing in
it could be deleted without sudo, and other copies came out writable
by every user. Copies from Windows now get 0755 folders and 0644 files.

**Dropping files from Explorer onto a file pane works again** with the
newer gunim, which says what a drop of files carries.

**A program asked something answered once.** A pane shown in another
window answered a second time, late, as if typed: cmd.exe saw ^[[?6c
in front of the next command.

**Starting a pane again is no longer reported as failed** when it
worked.

**A file dropped into a pane says so once,** and a path typed into a
pane on a Windows window reads C:\Users\…

**A dragged file leaves kakel** for a program whose window is in front
of a kakel window.

**A setting written by a newer kakel is kept,** rather than the whole
file being refused.

**Check for Updates in About** closes the dialog first.

## v0.4.0

### Changed

**Every question is asked in a window of its own.** A password, a
passphrase, a choice, or word that a server waits on opens in a small
window over the others, centred over the window you last worked in,
with the keyboard in it. One asked from a file manager window, or
with kakel in the tray, opens no kakel window. Its title bar moves it
aside. Enter answers, and Escape or Cancel cancels.

**The Servers pane shows each machine as a card.** A card has a badge
in the machine's own colour, its name, who and where it is, and its
route. A pill says how its connection is doing: for a connected server,
its round trip and how long it has been connected. Terminal and Files,
or Connect, are on the card, ⋯ holds the rest, and what is open on the
machine is listed under them. Cards stand side by side under This
Computer, Connected and Saved, and narrow into one line each when the
pane is narrow. A field along the top finds a machine as you type, and
Enter connects to what you typed when it reads as an address. Add holds
Add Server, Quick Connect and Connect to Window. The arrow keys move
along the cards; Enter, F, E, Delete and Shift+F10 work on the card in
focus, and / goes to the field.

### Added

**A file manager, in windows of its own.** File › Open File Manager, a
server card's Files, the launcher and the tray open this computer's
files in a window built on gunim's Files: a sidebar of places, a path
bar, a filter, a details panel, list and icon views. Its places list
this computer's folders, then each saved server with how it is doing.
It keeps to windows of its own, outside kakel's tabs, and kakel runs
while one is open. Files open where you last chose: the machine menu
has Files in a Window and Files in a Pane, and Files remembers which.
A window's title names the machine first, as "Picard — Documents —
kakel".

- **Servers' files too.** A click on a server in the sidebar connects
  to it and turns the window to its files; Ctrl+click opens it in a
  window of its own. A connected server's icon is green, and its menu
  has Disconnect, or Connect. A machine reached through a kakel window
  opens the same way.
- **Copy and move between machines.** Drag items, or copy and paste
  them, between file manager windows on different machines: this
  computer to a server, a server to this computer, or one server to
  another. The window they go to shows the copy as its own, with its
  progress and speed and a Stop, and asks there whether to replace a
  file. kakel's Jobs list it too. A move between two machines that are
  one folder, as this computer and a server that is this computer, is
  refused rather than losing the files.
- **Favourites on any machine.** Pin a folder on any machine; every
  file manager window lists them all, a machine's menu and the palette
  list its own, and the launcher finds them. The folders saved for a
  server and for This Computer become favourites, once.
- **OneDrive's online-only files.** Through a kakel window on a Windows
  machine, files a cloud provider keeps online only are shown so, and
  aren't read for thumbnails or previews, which would download them.

**Add Saved Key.** Puts a key you already have on the saved keys, which
the server form offers: one of the private keys in ~/.ssh, listed, or
any other, by typing its path. On the SSH Keys menu and the palette.

**Use Secret (Ctrl+Shift+S).** Pick a secret and type it into the
terminal used last, or copy it, without opening the Secrets pane. The
secrets for the machine of the pane in front come first, and a field
finds among them all; locked, the secrets are asked to open first. The
palette offers only Use Secret, Manage Secrets and Lock Secrets for the
secrets; the rest is in the Secrets pane.

**Import from SSH Config.** On Add and the Servers menu, it saves the
machines ~/.ssh/config names that aren't saved yet, with their host,
user, port, key file and ProxyJump.

**kakel starts in the tray.** Started from its shortcut, or with
nothing to do, kakel goes to the tray and takes the launcher's key; a
window opens from the tray, the launcher, or kakel started again. The
first time, a pop-up says where it went. Where there is no tray, a
window opens at once.

**A password asked for interactively can be kept in the secrets.** A
server that asks for its password through keyboard-interactive sign-in
offers to keep it, as the plain password question does, and the
secrets answer it from then on.

**Dialogs have a title bar, and blur what is behind them much less.**

### Fixed

**Exit asks only about what it would lose:** copies running, terminals
and tunnels. A connection with nothing on it closes without a question,
and so does a window with one pane.

**On Windows, a drag drops on the window in front,** not on a kakel
window behind it, and a file can be dragged from a window behind
another without bringing it forward, as in Explorer.

**The file manager's menus follow the pointer on Windows.** An open
menu's shadow lay over the menu bar and took the pointer, so the bar
missed moves and clicks; and a click on one title could flip the menu
between it and another.

**A caret doesn't blink in a window without the keyboard,** so a field
there no longer looks ready to type into.

**Copies to and from servers are many times faster on a slow link.**
A copy read and wrote 64 KB at a time, each waiting a full round trip
to the server, so a server 50 ms away gave about 1.3 MB a second however
fast the line. Copies now keep many requests on their way at once. Over
a 20 ms round trip, 8 MB goes in about a fifth of a second either way,
where it took 2.8 s down and 5.4 s up.

**A server's own key is tried first.** A key chosen for a server was
tried only after the SSH agent's keys, so an agent that didn't answer
held the connection up for ten seconds before the key was tried. The
chosen key now goes before the agent, and an agent gets three seconds to
list its keys, not ten.

**Locking the secrets locks them.** Lock forgot what the secrets hold,
but kept the key that opens them unlocked, so they opened again at the
next click or the next look for a saved password, with no question. Lock
forgets that key too. One the SSH agent holds can't be made to ask
again; kakel says so.

**The server cards answer as buttons and keys should.** Their buttons
are gunim's, pressed on release with the keyboard's ring, and every card
has Terminal, Files and ⋯; Connect, which opened a terminal too, is gone.
Enter on a card opens the menu of what opens on it, the shells among
them, so the keyboard picks; Enter twice opens a terminal. The arrows go
to what is above, below or beside a card on the screen. Ctrl+PageUp and
Ctrl+PageDown go on to the next tab from the Servers pane. Enter in the
search field goes to the first machine found, and the field says / finds.
Cards slide to their new place as a server connects or disconnects, and
fade in and out as they come and go.

**A key's passphrase question is easy to read.** It names the key in
bold, with the comment it was made with under it, and its folder only
when that isn't ~/.ssh. Opening the secrets for a sign-in, it says what
they hold the sign-in for and which key opens them, each on a line of
its own. A wrong passphrase is said in red.

**Locked secrets are asked to open for a connection they hold the
sign-in for.** A key's passphrase or a server's password saved in the
secrets was only used when the secrets opened without asking. Now, when
they are locked and hold it, kakel asks to unlock the secrets first, and
then signs in with what they hold. kakel remembers which logins and keys
the secrets hold something for as hashes, which name none of them.

**The window no longer crashes when a pane's shell goes first.** A
terminal pane closed while the window was still showing it, as a
connection's is when the connection is given up, crashed the window.

**No windows flash up as kakel starts on Windows.** The release is now
linked as a windowed program, so Windows gives it no console window to
close. wsl.exe, which kakel asks for the WSL distributions, and the
command that opens a link in the browser start without windows of their
own too.

**Shortcuts work after a click on empty room.** A click under the list
in the Servers pane took the keyboard from everything, and no shortcut
worked until a pane was clicked. The click gives the Servers pane the
keyboard now, and with nothing focused the window's shortcuts still work.

## v0.3.0

### Added

**Sign in with the secrets.** When kakel asks for a server's password or
a key's passphrase, it offers your saved secrets to answer with, and a
box to save what you type. Either is kept once the connection goes
through, so a password the server refused is never saved. From then on
that login or key signs in without asking. A saved secret is tried
first, and if it is refused you are asked again on the same connection.
A login is user@host, with the port when it isn't 22. A secret shared by
several logins is never changed by a sign-in: a new password typed for
one of them is saved on its own.

**A mistyped password gets another try.** A refused password is asked
for again, up to three times, rather than ending the connection.

**This Computer's own settings.** Options › This Computer…, also on the
This computer menu in the Servers pane, sets the folder new terminals
start in, the shell they run, and folders to open files at, as Edit This
Server… does for a server. The start folder is used when kakel was
started from a shortcut, the Start menu or with the computer. Started
from a shell in a folder of your own, kakel opens there as before.

**Install kakel, and keep it up to date.** `install.ps1` on Windows and
`install.sh` on Linux fetch the newest release, check it, and install it
for you alone. A copy you downloaded does the same from Options ›
Install kakel…, or `kakel -install`. On Windows that is
`%LOCALAPPDATA%\Programs\kakel`, a Start menu shortcut and an entry
under Installed apps; on Linux `~/.local/bin/kakel` and a desktop file.
Options › Start with Computer starts it into the tray when you log in.
The installed copy looks for a newer release once a day, and Options ›
Updates… says whether it tells you, installs it by itself, or does
nothing. Check for Updates now offers the update itself, not only its
page. `kakel -tray` starts into the tray, and `kakel -quit` ends the kakel
running. On Windows none of this has yet been run on Windows itself.

**A launcher on Shift+Win+K.** From any program on Windows, and on
Ctrl+Alt+K under X11 on Linux, the key opens a small window that finds a
machine, a shell (cmd, wsl), files on a machine, a saved command or a
kakel window as you type; Enter opens a terminal there, or what was opened
there last since kakel started, and Tab lists the rest. Options › Launcher Key… changes the
key. Open Launcher does the same from the Servers menu and the tray, and
`kakel -launcher` from a key the desktop binds, as under Wayland. macOS
has no global key yet.

**kakel in the tray, one at a time.** An icon in the system tray lists
this computer and the saved servers, each with what can be opened on it,
and works with no window open. Closing the last window leaves kakel
running there, until Exit or Quit kakel. A kakel started meanwhile hands
its command line to the one running, which opens a window for it; set
`KAKEL_ALONE=1` to start one of its own. Options › Tray Icon turns the
tray off. On Windows the icon is new code that has not yet been run on
Windows itself.

**File panes work as a file manager does.** Files show their kind as an
icon, and Icon View (Ctrl+2) shows a folder as tiles with thumbnails of
its pictures. Files drag between file panes in any window, moving on one
disk and copying elsewhere, and a folder a drag rests on springs open.
Files drag in from other programs, and files on this computer drag out
to them. A move between two drives now copies and deletes, where it used
to fail.

**Tool windows.** Open Servers Window and Open Secrets Window give those
panes a window of their own, and Move Tab to New Window does it for any
tab. A pane opened from such a window opens in the window last worked
in, which comes to the front. Asked for again, the Servers pane or the
secrets are shown where they are rather than pulled into the window
asking.

**Tabs.** A window holds tabs, each a pane or a split, and the tab bar
shows in the title bar once there are two. Drag a tab to reorder it, onto
another window to move it there, onto a pane to split beside it, or out
of every window to open a window of its own. Ctrl+Shift+PageDown and
Ctrl+Shift+PageUp move the tab in front.

**The menus behind one button.** The title bar reads kakel's icon, a
menu button, "kakel" and the pane in front. The button lists the menus,
and each opens beside the list. Everything else on the title bar moves
the window, so a window mostly off the screen can still be dragged back.

**More than one window.** In the grid of every pane (`Ctrl+Shift+A`), a
pane dragged onto another kakel window moves there, and one let go
outside every window opens a window of its own. Each window has its own
panes and sidebar. A window left empty closes, and closing a window with
panes in it asks first.

**Icons, from Lucide.** Rows, close crosses and plus buttons are drawn
with Lucide's icons, in place of pictures built from rounded
rectangles. The menus and the command palette show an icon beside each
command, the machine menu beside each thing it opens, and the tunnel,
secrets and jobs panes on their buttons. A question shows one before
its title: a shield on "Trust this server?", a key on a passphrase, a
plug pulled on a lost connection. A question that can do harm shows a
warning.

**A notice says whether it is a failure or work done.** A failure's
toast has a red alert icon, and work done a green tick. Other notices
stay plain.

**The secrets open in a pane, and can be taken out again.** `Manage
Secrets` lists what is in the vault and the keys that open it, and is
where a secret is copied, read, changed and removed -- several at once,
without a dialog that goes away on the first pick. `Show Secrets` stays
as it was, because `Type` sends a secret to the program in the pane in
front and only a dialog drawn over that pane knows which one that is.

`Export Secrets` writes every secret to a plaintext CSV, and
`Import Secrets` reads one back. A password manager nobody can leave is
one nobody should adopt, so the way out is plain text -- that is what
every other manager reads -- in the columns a browser writes, which is
the nearest thing to a standard there is. The import knows the headers
Chrome, Bitwarden, LastPass, KeePassXC and 1Password write, because
none of them agree. Both say to remove the file afterwards: it is the
one place every secret sits in the clear.

**A passphrase can open the secrets, for when every key is gone.**
Every slot was an SSH key, so losing them all lost the secrets, and
copying the file did not help because the copy wanted the same keys.
`Add Secrets Passphrase` adds a slot opened by something known rather
than something held, and then a copy of the file is a backup that
survives losing the lot.

Nothing adds one. It is the weaker door -- an ed25519 key is a hundred
and twenty-eight bits in a file, and a passphrase is what somebody
typed -- so it is a command, the dialog says what it costs before it is
added, and it opens on `Cancel`. What a guess costs is written into the
slot, so it can be raised later without shutting anybody out of a vault
sealed under the old cost.

**An agent can type, press and wait in one call.** `send_keys` takes a
list of steps -- `type:`, `key:`, `wait:<ms>`, `until:<text>` and a bare
`until` -- and works them in order. What it is for is what does not
happen between them: a command, the program it starts, and what is typed
into that program used to be three calls with a gap in each, and in
every gap the pane could be something other than what the agent thought.
Driving vim to write a three-line file took an agent nineteen calls, and
takes six now, with the whole editing session in one of them. A step
that does not do what it says stops the list there and the answer says
which one, how the waiting ended, and what the pane looked like, so the
steps after it never go to the wrong program. A wait for text has to
have seen that text: one that ended because the command finished
instead -- `cd somewhere && vim notes.md` with the directory wrong --
stops the list rather than typing an editor's keystrokes at a shell
prompt. `require:<text>` and `fail:<text>` are the same check asked for
directly: go on only if the pane says this, or stop if it does.

**A list of steps answers with what each of its waits saw.** A list can
run several commands -- type, Enter, until, then the next one -- and the
answer carries each one's output, headed by the step and what that
command exited with. The alternative is what agents do without it: chain
three commands on one line with semicolons, where the outputs run
together and a single exit status covers all three. That is also what
leads an agent to clear the screen before every command so that what
comes back is only its own, which throws away what the user had in front
of them.

**A wait can watch for text to arrive rather than text being there.**
`wait_for` matches the screen as it already is, which is right for it:
sending keys and waiting are two calls, and anything short has finished
before the second one lands. But the text an agent waits for is usually
a word it just typed, and a terminal echoes what is typed -- so "wait
until it says done" after `echo done` ended before the command had run.
`since_keys` waits for the text to arrive instead. Inside a list of
steps it is always on, because a list has no gap in which the text could
arrive unseen.

**A vault for passwords and notes, opened by a key you already
unlock.** `Show Secrets` lists what is in it; `Add Secret` and `Add
Note` put things in; `Change Secret` and `Remove Secret` deal with what
is there. It is a file called `secrets.json` beside everything else the
window saves, sealed with a key of its own, and that key is wrapped once
for each SSH key allowed to open it. So `Add Secrets Key` lets a second
machine's key in without re-encrypting anything, and `Remove Secrets
Key` takes one away with only its own wrapping.

The key has to be ed25519, because a slot is opened by having the key
sign a fixed challenge and only ed25519 signs the same way every time.
It is one of the keys already unlocked to reach a server, so the vault
usually opens for nothing: the passphrase typed at the first connection
is the whole of it. `Lock SSH Keys` locks the vault with them.

No secret is ever drawn on a row. `Copy` puts one on the clipboard and
says so without showing it, and takes it back off thirty seconds later
unless something else has been copied since. A window closing does the
same thing there and then, because the timer that would have done it
posts work nobody is left to run. `Type` sends it to the program in the
pane in front, so it never reaches the clipboard at all; a return goes
with it only when something is waiting for a whole answer. `Show` is the
only thing that puts a secret on screen, and it has to be asked for.

**A password gridterm makes, and the machine in front of you
suggested.** `Generate` on the add and change dialogs writes twenty
characters and leaves the dialog open so they can be read back with
`Show`. They are letters and digits with the lookalike pairs left out --
no `l` or `1` or `I`, no `O` or `0` -- because a password kept in a
vault is still read aloud now and again. No symbols: a symbol buys about
as much as one more character does, and it is the thing a server's own
rules refuse.

The `For` field starts on the machine the user is looking at and offers
the rest of the machines the window knows of. It is a suggestion in a
field that can be cleared.

**A new SSH key can have its passphrase generated and kept in the
vault.** `New SSH Key` has a `Generate passphrase` tick. With it on,
gridterm makes the passphrase, locks the key with it and puts it in the
vault, and nobody is shown it -- there is nothing to write down and
nothing to lose. Unlocking that key from then on takes the passphrase
out of the vault instead of asking. One that the key refuses falls
through to the dialog, and a key the vault knows nothing about is asked
about the way it always was.

It only reads a vault that a key already unlocked opens: asking for a
passphrase to read a passphrase would be a dialog to spare a dialog, and
the key being unlocked may be the vault's own. The tick is only offered
where there is a vault to put one in.

A key whose passphrase is in the vault is no use as a spare for it: with
the other key gone, opening the vault needs this key and unlocking this
key needs the vault. `Add Secrets Key` marks that key in the list and
says so before adding it, and adds it anyway when told to -- the
passphrase can be copied out and kept elsewhere, and then it is a spare
like any other.

**A word before the secrets are trusted to a key the SSH agent has.** A
slot is opened by that key signing, and an agent signs for whoever it
is forwarded to, so `A server you forward the agent to can open any
copy of the secrets it has.` Starting a vault on such a key, or adding
one to a vault that exists, now says that first and goes ahead when
told to: it is a cost only to somebody who has a copy of the file as
well, and whether that is worth it is the user's to weigh.

This and the warning about a key whose passphrase is in the vault are
one question, because a key can have both against it. It opens on
`Cancel`, the way every question about exposing something does.

The agent is asked by fingerprint, off the `.pub` file beside the key,
so nothing has to be unlocked to ask, and a key with no public half or
an agent that will not answer gets no warning rather than one the
window cannot stand behind. It is a snapshot: the key may be added to
the agent a minute later. The key `New SSH Key` offers by default,
`id_ed25519_gridterm`, is not one most people load into an agent, and
agent forwarding is off unless a saved server turns it on.

### Changed

**The Servers pane replaces the sidebar.** Every machine, with what is
open on it in every window, is now listed in a pane of its own.
`Ctrl+Shift+L` opens it in a tab or goes to it, and `Ctrl+Shift+B`
opens or closes it. Drag its tab out to keep it in a window of its own.
A click on a pane in another window brings that window to the front.
Quick Connect and Add Server are along its top. The stage takes the
whole window, and a terminal's text sits a little in from its edges.

**Servers are known by an ID, not their name.** Everything open on a
saved server, its connection, panes, files, log and tunnels, goes by the
ID the server list gave it. Renaming a server is saving its new name:
nothing moves, and nothing is refused.

**Quick Connect replaces "+ Connect to server…".** It is on the Servers
menu, with the same shortcut. A server connected to by typing its
address gets an ID of its own, is listed marked "quick", and is
forgotten once it is not connected and nothing is open on it.

**Ctrl+PageDown and Ctrl+PageUp go to the next and previous tab.** They
used to go to the next and previous pane in the sidebar. Next Pane and
Previous Pane are still on the Pane menu and in the palette, with no
shortcut. A shortcuts file written before this change still binds them
to Ctrl+PageDown and Ctrl+PageUp. Delete those two lines to get the tab
keys.

**gridterm is now kakel, and its window is drawn with gunim.** Kakel is
Swedish for tile. The window was drawn with ebitengine; it is now drawn
with [gunim](https://github.com/marrasen/gunim), a GPU toolkit built
around animation, and the ebitengine window and its fork are gone. The
program is `kakel`, the module is `github.com/marrasen/kakel`, and
`go install github.com/marrasen/kakel@latest` installs it.

Your files come along. The first time kakel starts, it renames
gridterm's directory to kakel's, and a copy that carried a
`gridterm-files` directory beside it carries it on as `kakel-files`.
Shortcut files keep working, since the commands kept their names, and a
kakel and a gridterm still connect to each other. An agent set up for
gridterm is set up again for kakel: the MCP server is now called kakel.

With gunim came motion and a few things of its own:

- The window fades in as it opens and out as it closes, and the pane
  switcher zooms from every pane into the one you pick.
- An echo goes out past the window's edges when something happens you
  may be looking away from: red for a failure, green for work done,
  amber for a bell, and a faint grey ring while a connection is made. A
  theme's `Echo` block sets its colours and strength.
- `F11` fills the screen with the pane or split in front, and brings
  the menus and the sidebar back again.
- A program that wraps its frames in synchronized updates, as termflix
  does, is shown a whole frame at a time, and one that asks whether the
  terminal knows a mode is told.

**What this window is serving is a pane too.** It was a dialog, which
could only say who was connected at the moment it opened -- and what it
is about changes while it is up, as windows connect and go. The pane
follows: the address, the fingerprint to check this machine by, and one
line per window working here, with `Disconnect` and `Stop serving`
along the bottom. The row of a window working in this one opens it,
which is what that row could not do before: it stood in for a pane
because there was none to put in front.

**A finished drop says so without a dialog.** A file dropped on a pane
put up a box when it landed, which took the keys from whatever the user
had moved on to. It is said the way a program's message is now: a line
in the log, a pop-up outside the window, and a line on the bottom row.
A file whose pane closed before it landed still gets a dialog, because
its path could not be typed and the dialog's `Copy` is how to get it.

**File work is watched in a pane, not a dialog.** Clicking a copy's row
on the sidebar opened a box over the window, which took the keys and had
to be dismissed before anything else could be done -- for work that
takes as long as it takes. It opens a pane now, and the pane has room to
say more than the box could: how far it has got, drawn as a bar that
moves in eighths of a cell; how much has moved, how fast, and how long
is left; the last seconds of it as a run; and the names it was given,
ticked off as it passes them. `Cancel`, `Repeat` and the box that keeps
a copy are along the bottom, and `Close` takes the pane away and leaves
the work running. `Repeat` watches the new run in the same pane, so a
copy done again and again does not leave a pane for every time.

**A connection's account opens in a pane, and the row of a connection
that dropped opens it.** `Connection Log` showed the account in a dialog
that had to be dismissed; it is a pane now, the way the window log is,
so it scrolls and copies like anything else on screen. The row of a
machine with nothing open on it used to answer a click with nothing at
all, and the row of one that dropped did the same -- so the moment there
was most to explain was the moment the window had least to say. Both
open the account now, and it outlives the connection it is about: the
reason a connection was lost is written into the account, where there is
room for it, rather than onto the row, where a sentence either pushed
the name off the end or did not fit and was dropped without a mark.

**The secrets went through the wording rules a second time.** Five
dialogs said `Take "Add Secret"` where the vocabulary table has
`choose` for a menu line. The `Show` button on the add and change forms
renamed itself to `Hide`, which rule 9 calls a bug wearing an
explanation; it is a `Show the secret` tick now, beside the field it is
about. The list `Show Secrets` opens was titled `Secrets` and the two
key lists both said `Choose a key`, so two different jobs shared a
heading; each is titled with the command that opens it. The question
before a key is revoked offered `Remove` and `OK`, where `OK` reads as
agreeing to the removal, and carried a `Copy` button over a body with
nothing in it to copy; it is `Remove` and `Cancel`. Two warnings that
ran to two sentences are one each. And the line over the add and change
forms said `Sealed in the vault`, which was the only place on screen
that called the secrets anything but the secrets; it now says `Only
your key opens the secrets.`, which is the one thing the title cannot
say. Every error the `secrets` package reports went the same way: they
said `the vault`, and they are read under a heading that has just said
`the secrets`. The ones that also said make, take or holds now say
create, create and is, which is what the vocabulary table has.

**A list's buttons look and answer like every other dialog's.** The row
along the bottom of a chooser drew its actions as names in square
brackets, so `Show Secrets` offered `[ Type ] [ Copy ] [ Show ]
[ Cancel ]` while every dialog beside it drew real buttons. They are now
the same buttons, right aligned from the corner the eye lands on, and
clicking one works. A form also takes left and right along its button
row, which a notice and a chooser already did -- so every dialog in the
window is answered the same way. In a field the arrows are still the
caret's.

**The File menu's shells say only which shell they open.** Under the
`New Terminal In` heading each line read `New Terminal: Command Prompt`,
so the heading and the line said the same three words before either got
to the shell. The lines are now `Command Prompt`, `Windows PowerShell`
and the rest, which is what the same list already said on a machine's
plus menu. The commands keep their full titles for the palette, where
they are read with no heading around them.

### Fixed

**Edit This Server shows what was saved.** The window took in a saved
server again only when its name or address changed, so an edit to its
user, port, key or folders showed the old values the next time.

**A bell's rings show on the window that rang, on Windows.** They used
to show above every window. A window behind another drew its rings over
the one in front, and over other programs too, so they looked like
another window's. Now they stay with their own window and go behind
whatever is in front of it.

**The hand shows over a link with Ctrl held.** The pointer over a
terminal is the I-beam, and the hand where Ctrl and a click would follow
a link.

**A link's address no longer sticks after a Ctrl+click.** A browser the
click opened took the button's release, and the link stayed lit, its
address shown at the foot of the pane, until the next click.

**A delete's question shows which button Enter presses.** It opens on
Cancel, with its ring showing, and the arrow keys move between the
buttons.

**A folder that can't be opened is said in a notice.** It sends a red
echo, as a delete sends a green one, and the Window Log keeps it. The
line under the path that said "Click for why" is gone. Failures the
window finds itself, such as files dropped where they can't go, are
kept in the Window Log too, with the same red echo.

**Select text in the scrollback.** A selection stays on its text as the
view scrolls and as output moves it up, rather than on the rows of the
screen. A drag held past a pane's top or bottom scrolls toward it, faster
the further past, and the wheel scrolls during a drag too, so a
selection reaches as far back as the scrollback goes. Edit › Select All
selects all of it.

**Tabs open and close in motion.** A new tab grows in where it lands,
a closed one shrinks away, and the tabs beside it and the + slide into
place. They used to appear and vanish at once.

**Ctrl+wheel zooms when it should on Linux.** Under X11 a scroll just
after pressing Ctrl scrolled instead of zooming, and Ctrl stayed held
for the wheel after it was let go, so scrolling zoomed. kakel now asks
the system what is held as the wheel turns.

**A bell echoes from its own window.** A bell, or a long command
finishing, in a pane of a window behind sent its echo from the window in
front. Each window now counts its own, and asks for attention itself.

**A window closing waits for file work the user dismissed.** Dropping a
copy cancels it and takes its row away, and cancelling is not stopping:
the write it is in finishes, and one waiting on a machine that has
stopped answering waits however long that takes. The queue stopped
answering for it the moment it left the list, so a window could close
while it was still writing.

**A click in the file browser points at a name, and a double click
opens it.** One click opened, which took the user into a directory they
had only meant to pick out. The click that moves the keys to a pane
moves the bar as well now, so a double click on a pane without the keys
opens what it was aimed at.

**The file browser says it is reading.** A pane with nothing to show
said nothing while its first listing was on the way, so a slow machine
looked like an empty directory. It says `reading…` until the listing
arrives.

**Ctrl+D on a file goes back to the browser.** Closing a file, or the
scrollback of a pane, gives the keys back to the pane it was opened
from. They used to land wherever the window's layout put them, which
with another pane open was not the browser.

**The search match being on is marked.** "/" finds and Next moves
between the matches, and every match looked the same. The one Next
steps from is drawn in the selected text's colour on the match's own,
and underlined.

**The strip beside a file is drawn, and can be dragged.** It was made
of block characters, which could say how full a band of lines was in
five steps and nothing finer. It is drawn in pixels now, a bar per row
as long as the lines it stands for, with a log's bad parts in their
colour beside them. Dragging it scrolls the file, the way a scrollbar
does, and the file keeps following the pointer off the strip until the
button comes up.

**A copy of gridterm on a USB stick can serve.** The key a window serves
with was put in place with a hard link, so two windows making one at
once could not write over each other's. FAT32 and exFAT have no hard
links, so the link failed every time and serving was refused. Where
there are none, the name is claimed on its own and the finished key
moved onto it. The same goes for a new SSH key made there. On Linux and
macOS such a stick gives every file the mode it was mounted with, and a
key that mode opens to others is refused as before, now saying to mount
it with `fmask=0077`.

**A starting file is written whole or not at all.** The keyboard
shortcuts file and the theme file were created and then filled, so a
write that failed part way left a short file, and the next attempt
refused to write over it.

**Connecting to another window opens nothing on it, every way in.**
Connect to Server with a window's name, Reconnect after a window's
connection dropped, and a file pane asking for its window back each
still opened a terminal over there. They connect and nothing more; a
terminal on a window is asked for from its plus.

**A shell on another window that ends can be reconnected.** It said the
program had finished and offered nothing. It asks "Connection closed."
with Reconnect now, and Reconnect has that window start its own pane's
shell again and watches it, so no finished panes pile up over there. A
window of an older build is asked for a new shell instead.

**A pane beside a WSL one starts where that one is.** A split on cmd
from a WSL pane failed with "The directory name is invalid", because it
was handed the Linux path. It is handed the Windows path of the same
place, and a WSL shell beside it is sent back inside.

**A folder with a zip in it is copied with the zip.** The browser shows
a zip as a directory to walk into, and a copy of a folder took that view
and copied what was inside each zip instead of the zip. A copy, a move
and a delete now take a zip for the file it is. Only a file is taken for
an archive, so a real directory called `x.zip` is a directory, and a
link to a jar is the jar.

**A zip on a machine behind another window can be walked into.** It was
listed as a plain file, because that filesystem was the one built
without archives opened.

**The top of a Windows machine lists the drives that answer.** Going
above `/C:` over a connection to another gridterm failed with "The
device is not ready" when one drive, an empty card reader say, could
not be read, and took every other drive with it.

**The cross on a file's row closes it.** It did nothing on a row for a
file being read, a tunnel, a piece of file work or any other row that is
not a terminal or a file browser.

**A dropped file's row is under the machine it is going to.** A file
dropped on a pane on a server showed its copy under this machine,
because that is where the file was read. Work is filed under the
machine it writes to now, unless that is this one.

## v0.2.1

### Fixed

**The line along the bottom row is no longer hidden behind the
sidebar.** The sidebar is drawn on a layer of its own over its columns,
and the line started at the first of them: `Shortcuts reloaded` is
shorter than the sidebar is wide, so it was invisible altogether, and so
was the sentence a menu puts there for the row under the cursor. It now
starts beside the sidebar. This is also what `Check for updates` says
when this build is already the newest release, so in v0.2.0 that answer
could not be read at all.

## v0.2.0

Every dialog and every command title reworded, and the about dialog can
now say which build it is.

### Added

**The about dialog says which build this is, and checks for a newer
one.** It said `Version: development build` whatever it was. It now says
what the build calls itself -- the tag for a release, `dev-<commit>` for
a build from a working tree -- and it can be copied, because that is the
first thing a bug report needs. Beside `OK` is `Check for updates`,
which asks GitHub for the newest release. A newer release opens a dialog
naming both versions and the page it is on, with `Open`. A build that is
already the newest release, or later than it, gets a line along the
bottom row instead of a dialog to dismiss. A build from a working tree
is not put in that order at all, because it may hold work no release
has: both versions are named and the choice is left to whoever is
reading. The check is manual, and nothing asks GitHub until the button
is pressed.

**What a dialog needs, instead of a paragraph explaining itself.** A
field carries one hint line, drawn along the bottom while that field has
focus. A field that does not apply is disabled -- dim, taking no keys,
with the focus stepping over it -- rather than taken and quietly
dropped. A field with a list of values draws `Ctrl+↑↓` beside itself. A
notice leaves `Copy` out when there is nothing worth copying, and the
palette draws a tick for a switch the way a menu does. And there is a
status line: one line along the bottom row for four seconds, for an
action that worked and has nothing to read.

### Changed

**Every dialog is reworded, and eight behaviours changed rather than
being excused.** Titles say what happened, bodies add only what the
title cannot, buttons are one verb from a small set, errors are two or
three words, and placeholders are a format or the word `Optional`. Where
a sentence existed to excuse a behaviour, the behaviour changed instead:
the passphrase dialog asks until the key opens or you cancel, rather
than counting down from three tries on a file you can already read; the
tunnel's direction is a field rather than two buttons explained in the
body; the overwrite question has a `Cancel`, which is what dismissing it
already did; the copy dialog's button that renamed itself between
`Remember` and `Forget` is a tick box; the server dialog's jump host and
agent tick are disabled where a window cannot use them, rather than
taken and then explained away; two dialogs that only said something
worked are status lines; and the shortcuts file's rules are in the file,
where somebody editing it is looking. `Find in Scrollback` opens with
the find prompt up, so its name is true when it arrives. A server's
mid-handshake message can open the one link it carries, when it carries
exactly one and that link is http or https -- never on its own, and with
the focus staying on `Close`.

**Every command has a new title.** Title Case, verb first, two to four
words: `Close Pane`, `Reload Themes`, `Open Tunnel…`. An ellipsis means
the command asks something before it acts. Every word a title dropped is
still typed into the palette to find it, so whoever learned the old
wording still gets there. Command ids do not change, so a saved shortcut
still points at the same thing. A command that fails is headed by its
own title -- `Open Tunnel failed` -- and every error heading has one
shape: `Could not <verb> <object>`.

**The sidebar's rows say what a connection is, in the same words
everywhere.** Three were sentences that also named the window the row
sits under: `the window stopped sharing` is `stopped sharing`, `the
window closed this connection` is `closed by that window`, and `no
longer serving` is `connection lost` -- which is what a dropped
connection says whether it was carrying one pane or a whole window.
`given up on` is `cancelled`.

**The wording rules are written down.** WORDING.md is how a dialog,
button, menu row, command title and error are worded; DIALOGS.md and
COMMANDS.md record what those rules were applied to. Every button title,
field label and dialog title is a constant in one file, so no word can
drift from a second copy of itself.

### Fixed

**A wrong passphrase is asked about again, and said out loud.** Typing
the wrong passphrase for a private key used to be silent: the dialog
closed, the key was never offered, the connection went on to whatever
else it could try, and every line in the account stayed green. It is now
asked for again, with the dialog saying that the last one did not open
the key, until the key opens or you cancel. A wrong passphrase no longer
falls through to another way of signing in, and the key is named in what
the connection failed with -- even when the connection was made some
other way in the end.

**A window that stopped sharing is no longer reported as a connection
that dropped.** A window says so down the control channel before it
goes, so the other end can tell a deliberate stop from a connection that
broke. That line could be thrown away before it was read: the row then
said the connection was lost, with a reset beside it, and the window
offered to reconnect -- for something the user had done on purpose. The
window now waits for the other end to say it has the line.

## v0.1.0

The first release. Windows and Linux, on amd64.

### Added

**The terminal.** A GPU-rendered character grid: a full screen of text
is typically two `DrawTriangles` calls, and a screen that did not change
draws nothing at all. Alternate screen, scroll regions, scrollback, 256
and true colour, the text attributes, cursor shapes, device reports,
bracketed paste and mouse modes. bash, vim and less all run. Wide
characters take two columns, combining marks share a cell, and the box
and block characters are drawn at the exact cell size so a framed TUI
has unbroken lines.

**Keyboard.** Press, release and OS repeat with modifiers, correlated
with the text a keystroke produced, encoded to the bytes a program
expects -- including application cursor mode, which vim and readline
need.

**Shells here and on other machines.** A local pseudo-terminal, a ConPTY
on Windows, or SSH. One connection carries several things at once, so a
second terminal on a machine is a second channel rather than a second
login. A saved server can sit behind another one, and the second
connection rides inside a channel of the first.

**A file manager, with as many panes as you want.** Panes on this
machine and on any server at once, keyboard-driven the way a two-pane
browser has worked for thirty years, with copy, move and delete running
in the background and saying how far they have got. A name that is
already there is always asked about. Files land whole or not at all.

**A file reader without a shell.** Paging, search, go-to-line and a hex
dump, on this machine or a server. Code is coloured by what the file is
called, markdown gets its headings, and a picture file shows the
picture. A file can be tailed as it grows. A strip beside the file shows
the shape of the whole of it, with a second column for how bad a log
line got.

**JSON logs read as logs**, laid out in columns with the level coloured,
reading the field names every logger spells differently.

**Links and paths in the output.** Ctrl and a click follows an OSC 8
link, an address in the text, or a file the output named -- including on
a server, and at the line a compiler named. A path is checked against
the disk before it counts as a link.

**Pictures in a pane.** OSC 1337, on a layer of its own, scrolling with
the text.

**Files dragged into a pane**, landing in the directory the shell is in
rather than being typed, copied to the far machine first when the pane
is on one.

**Tunnels**: local, remote and SOCKS5, with a pane for each showing what
it is carrying and a way to watch what goes through it. Anything that
would let the rest of the network in asks first.

**One gridterm working inside another.** A window serves itself on a
port you opt into; another window takes it over, draws its panes, reads
its files over the same connection, and can join a program already
running there so both people see it. Key authentication only.

**Panes shared with an agent**, over the Model Context Protocol
(`gridterm -mcp`). One code lets an agent read and type into exactly the
panes you shared and nothing else. Nothing listens until you share a
pane, and taking the last one back makes the code useless at once.

**The WSL filesystems**, browsable like any other directory.

**A sidebar** rather than a row of tabs: every terminal, file pane,
tunnel and transfer under the machine it is on, with saved servers
listed whether or not anything is connected.

**Shell integration** that teaches bash, zsh, PowerShell and the Command
Prompt to say where they are and how each command went.

**Secrets asked for in the window** -- key passphrases, passwords,
one-time codes -- held in memory only. Unknown host keys are shown with
their fingerprint and recorded only on an explicit yes; a key that
changed is a hard failure. The SSH agent is carried to a machine only
when you turn it on for that machine.

**Linux support.** The window, the clipboard and the fonts all work on
X11. On a Wayland desktop it runs through XWayland. See
[LINUX.md](LINUX.md).

**Nothing to install to build it.** Every build is pure Go:
`CGO_ENABLED=0`, no C toolchain, no development headers, and each
platform cross-compiles from the other. The Linux binary asks for no
versioned glibc symbol at all, so it runs on far older distributions
than a cgo build would. arm64 builds for both platforms, though nobody
has run one. gridterm carries a fork of ebitengine for the key-event
pipeline; it is 404 lines on top of upstream v2.10.2, and **The ebiten
fork** in [BUILDING.md](BUILDING.md) says why.

**WSL panes are given paths their distribution can open.** A file
dropped on one, and a picture pasted into one, are named the way that
distribution names them -- `/mnt/c/...` rather than a Windows path the
program in the pane cannot open.

### Known gaps

No release for macOS. [GAPS.md](GAPS.md) says what else is not there
yet.

The cursor in a pane on another machine is held on for a fifth of a
second after a program hides it, so that the hide and show a repaint
makes never reaches the screen. It fixed a flicker that neither of the
two people who looked at it could reproduce directly, and it rests on
tests of the mechanism rather than on having watched the symptom go. If
a cursor lingers where it should not, that is the thing to suspect.
