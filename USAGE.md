# Using kakel

Keys, the shortcuts file, sharing panes with an agent, and where things
are kept. [FEATURES.md](FEATURES.md) says what each of these is for.

## Keys

Kakel comes with:

| Key | |
|---|---|
| `Shift+PageUp` / `Shift+PageDown` | scroll the scrollback |
| mouse wheel | scroll, or arrow keys on the alternate screen |
| drag | select; `Alt+drag` selects a rectangle |
| `Shift+drag` | select even while a program owns the mouse |
| `Ctrl+Shift+C` / `Ctrl+Shift+V` | copy and paste |
| middle click | paste |
| `Ctrl+=` / `Ctrl+-` / `Ctrl+0` | font size, from 8 to 96 pixels |
| `Ctrl+Shift+B` | open or close the Machines pane |
| `Ctrl+Shift+L` | go to the Machines pane |
| `Ctrl+Shift+N` | connect to a server |
| `Shift+Win+K` on Windows, `Ctrl+Alt+K` on Linux | the launcher, from any program |
| `Ctrl+Shift+A` | show every pane at once |
| `Ctrl+PageDown` / `Ctrl+PageUp` | the next / previous tab |
| `Ctrl+Shift+PageDown` / `Ctrl+Shift+PageUp` | move the tab right / left |
| `F11` | fill the screen with the panes |

A paste of more than one line, or over 5 KB, opens in an editor first.
The editor colours the code the way the pane's shell reads it: bash
(and other POSIX shells), PowerShell or the Command Prompt. The text
stays plain while a program the shell started is running, and in a
pane opened through another kakel window.
`Enter` pastes it, `Shift+Enter` starts a new line, and `Escape`
cancels. Settings › Terminal › Show before pasting turns this off.

A file manager pane showing a folder of a git repository on this
computer says the branch, how far it is ahead of or behind its
upstream, and how many files have changed, on its line along the
bottom. Where the repository's own settings name a program for git to
run, as a filter, only the branch is shown. Folders on servers show
nothing.

These are the ones you need most. "Shortcuts and Commands" on the Help
menu, or `Ctrl+Shift+H`, lists every command, the key that runs it, and
the name the shortcuts file calls it by, then the keys of the reader.

## Changing a shortcut

To change a shortcut, choose Settings › Files › Shortcuts › Start the
File. It writes `keys.json` holding every shortcut you have now. Edit
it, then choose Read Again beside it.

The file says what to change, not what the whole window does:

- Add a line to put a command on another chord. To move it, set the old
  chord to `"nothing"` as well, or the command runs on both.
- Delete a line and that chord goes back to what kakel comes with.
- Shortcuts added to a later kakel arrive on their own. A built-in
  chord that a later kakel moves does not, because your file still
  names the old one.

Every chord in the file runs before the pane sees it, so a chord a
program in the pane needs stops reaching it. A chord you would type,
such as a plain letter, is refused for that reason: hold `Ctrl`, `Alt`
or `Super`, or use a function key.

The file browser's own keys are not in the file.

## Every pane at once

Pane → Go To → All Panes…, or `Ctrl+Shift+A`, shows every pane of
every kakel window at once, each one live and shrunk, over the whole of
the screen the window is on. Each window shrinks from where it stands
into a card, and each of its tabs sits in the card the shape of the
window, its splits as they stand. Windows on other screens fly in from
their side. The arrows walk the panes, `Tab` goes through them in turn,
`Enter` goes to the one marked and `Escape` puts every window back as
it was. A click goes straight there: the pane grows back into its
window, which is in front, on that tab, once All Panes has gone.
`Ctrl+Shift+A` again closes it. The launcher (`Shift+Win+K`, or `Ctrl+Alt+K` on Linux) offers it
too: type "all panes" and press `Enter`. Where kakel cannot say where its
windows are, All Panes shows over the window it was asked in instead. The images are shrunk by the GPU rather than cell by
cell, and a window with nothing happening in it still skips the frames
it would have skipped anyway.

## More than one window

In All Panes, drag a pane to move it:

- onto a pane, and it joins it in a split, on the side of it you let
  go nearest; the half it would take lights up.
- onto another window's card, and it moves there on a tab of its own.
  Onto its own window's card, it leaves its split for a tab of its own.
- onto room no card covers, and it opens a window of its own where you
  let it go. A window's only pane stays where it is.
- over another kakel window, and it moves there; that window lights up
  while the pane is over it.

In a split, each pane has a line along its top naming it; the pane with
the keyboard has a coloured edge under its name. A click on the line
gives the pane the keyboard. Drag the line to move the pane, without
All Panes:

- onto another pane, in this window or another, and it joins it in a
  split, on the side of it you let go nearest; the half it would take
  lights up.
- onto the tab bar, and it leaves its split for a tab of its own, where
  you let it go. With one tab, and no tab bar, let it go above the
  panes.
- outside every kakel window, and it opens a window of its own there.

The Machines pane, the sidebar, has its own title and no line.

Each window has its own panes and its own pane in front. The Machines
pane lists the panes of every window, and clicking one in another
window brings that window to the front.
The pin in the title bar, before minimize, keeps a window above other
programs' windows; View › Always on Top does the same. Questions and
notices show in the window you last worked in. A
window whose last pane moves away closes. Its close button asks before
closing the panes still in it, and the last window's asks as Exit does.

## Sharing a pane with an agent

Click into a terminal and choose Share → Share Panes…. The first time,
that starts a share with that pane. After that it opens the Agent Share
dialog: the one code, a tick box for each terminal pane that puts it in
the share or takes it out, and which agent the prompt is for -- Claude
Code, Codex, Cursor, or another host that takes a JSON MCP config. The
agent picked is remembered for next time. The "Agent Share" chip on the
menu bar opens the same dialog.

The dialog's buttons:

- **Copy Prompt** puts a prompt on the clipboard and does nothing else.
  Paste the whole of it to the agent. It carries the code and says how
  that host adds kakel's `kakel -mcp` server. What the tools do and
  what the rules are come from the server's own instructions once the
  agent connects, so the prompt does not repeat them.
- **Copy Code** copies the code alone.
- **Setup…** shows the command line, or the JSON for a host set up by a
  file, that adds the server, with a button to copy it.
- **Write Skill** saves a `SKILL.md` where that host reads skills from,
  and says where it went. `kakel -mcp-skill` prints the same file.
- **Stop Sharing** ends the share, and the code stops working. Taking
  the last pane out does the same.

Share → Permissions… shows four tick boxes for the pane you are on,
saying what the agent may do there beyond reading the pane and typing
into it: read only, read cleared scrollback, open more panes, and
restart the connection. Every box starts off, and a change takes effect
at once. An agent that hits a password prompt can ask you to type it
into the pane: a line appears saying what it wants, what you type goes
to the program, and the agent is told you typed something and never
what.

Kakel writes down what an agent types, and Share → Typing History shows
it for the pane you are on. You hand the pane over, you give the access
and you hold the secrets, so what the agent does in there is yours to
read. It is what the agent sent, not what the shell ran: a line it
edited before pressing Enter is there as it was typed. A secret you type
at the agent's asking is not in it, because you typed that yourself.
The record lives as long as the pane: it is capped, it says how many of
the oldest entries it has dropped, and it goes when the pane closes.

## Serving this window, and working in another

Serving this window and connecting to another are on the Share menu
rather than on a key. "Serve This Window…" asks for the port and where
to listen, and then shows the address and the host key's fingerprint to
check. "Connect to Window…" asks for the address and the key file to
offer. Connecting opens nothing over there: what
that window has open lands in the Machines pane under its name, and the plus
on that heading opens a pane on it. The servers that window is
connected to get headings of their own, and their plus opens things on
them through it. Nothing listens until you ask it to,
and the keys allowed in are the ones you list in an `authorized_keys`
file in kakel's own directory, not the one in `~/.ssh`. The Serve dialog
lists them: Add Key… takes a public key pasted, or one of this
machine's, and Remove Key… takes one off and hangs up on the window it
let in. If you made that file a link to `~/.ssh/authorized_keys`, the
keys added here may log in over ssh too.

### Through a relay

When the other machine cannot reach this one, serve through a relay
(see below). Tick "Through a relay" in the Serve dialog and paste the
relay's URL. Kakel makes a code, such as `7KQ2-M9XD`, and keeps it in
`relay_code` beside the serving key, so it is the same every time this
window is served and each machine has its own. The other end types
that code. Copy Link puts the relay's URL and the code on the
clipboard, for pasting at the other end in one go. New Code makes
another; the old one lets nobody else in. Untick "On a port of this
machine" to serve through the relay alone. The keys that may connect
are the same either way.

At the other end, "Connect to Window…" with "Through a relay" takes the
relay's URL and the code; pasting the copied link into either field
fills in both. The relay used last is offered again. A window saved in
the server list as a kakel window takes the relay's URL as its address
and the code in Code. The host key is checked as for any window, under
the code and the relay's name: a new code is a new window to trust.

Both dialogs have a Through choice: a saved server to reach the relay,
or the window itself, through, as a server saved behind another one is
reached. The stream goes out from that server over the SSH connection
to it, which kakel makes first when it is not connected. A saved kakel
window takes a Through as well.

Kakel reaches a relay through the HTTP proxy that `HTTPS_PROXY` names
or, on Windows, the one set for the system. Settings › Sharing ›
Relays › Proxy overrides both: a URL such as
`http://user:password@proxy:8080`, or `none` to go straight there. A
proxy found only through a PAC script, or one that signs in with
Windows (NTLM or Kerberos), is not supported yet.

### Tunnels with plain ssh

A served window also takes ssh's own tunnels, with the same keys and
the same host key. There is no shell, so give `-N`:

    ssh -p 2222 -N -L 8080:intranet:80 kakel@office-pc

That makes port 8080 here reach `intranet` port 80 as the served
machine finds it. `-J` and `-W` work the same way. The user name says
where the tunnel goes out from: `kakel` for the served machine itself,
or a machine saved there, by its name as the commands spell it
(`db-server` for "DB Server"):

    ssh -p 2222 -N -L 5432:localhost:5432 db-server@office-pc

A saved server not connected over there is connected to, when that
needs nothing asked: a key, or a secret while the secrets are open. A
question would wait on a screen nobody may be at, so the tunnel is
refused instead, saying to connect it there first. A kakel window saved
there is gone through only while it is connected.

Through a relay, ssh runs kakel to reach it:

    ssh -o ProxyCommand="kakel -relay-pipe https://relay.example.com/k/…#7KQ2-M9XD" -N -L 8080:intranet:80 kakel@office-pc

`-relay-pipe` takes `-proxy` for an HTTP proxy, or uses the system's.

"Tunnels with ssh…", on the Share menu, in the served dialog and in
Settings › Sharing, explains all this with the commands filled in for
this window: its port, its name, its relay and the machines saved here,
each with a Copy beside it.

## Running a relay

A relay joins two kakel windows that cannot reach each other: both
reach the relay instead, over WebSockets on HTTPS, which gets through
HTTP proxies. It runs on a server with no screen:

    kakel -relay -domain relay.example.com

That gets a certificate from Let's Encrypt, so ports 443 and 80 must be
reachable. Behind nginx or Caddy doing HTTPS, run it on plain HTTP
instead and point the proxy at it, passing WebSocket upgrades:

    kakel -relay -listen 127.0.0.1:8080

It prints its URL, ending in `/k/` and a secret. Without the secret the
relay answers nothing, so keep the URL to the people who should use it.
The secret is kept in `relay_secret` in kakel's private directory (or
`-dir`); `-new-secret` makes a new one and the old URL stops working.

The relay only carries bytes. The windows speak SSH to each other
through it, so it cannot read what passes, and the served window still
lets in only the keys it lists.

## Where kakel keeps its files

Kakel keeps its files where the operating system puts a program's.
Make a directory called `kakel-files` beside `kakel.exe` and it
keeps them there instead, so one machine can hold several copies with
files of their own. Settings › Files names every file and says how to
move them.

Kakel used to be called gridterm. On its first start it renames
gridterm's old directories: `gridterm` becomes `kakel`, and
`gridterm-files` becomes `kakel-files`.

## The file manager

The file manager is a pane, like a terminal: split it, put it in a tab,
or drag it to another window. Files on a server's card, or the Files
command, opens one beside the file manager in front; Files in a New
Window opens one in a window of its own. Files from the launcher or the
tray opens a new tab in the window you last worked in; Settings ›
General › Files open in can make it split beside the file manager
there, or open a window of its own. Opening a folder from Windows,
once kakel opens folders, opens a window with one. A window of file
managers alone opens where the last one was, and as big, as it closed.

kakel's menus are its menus. While a file manager is in front, its
lines join kakel's File, Edit and View menus, a Go menu comes after
View, and the lines for a terminal alone leave. Edit's Cut, Copy, Paste
and Select All act on the files, File › New File Manager Window opens
another in a window of its own, and Close Pane closes it. `Enter` opens,
`Backspace` or `Alt+Left` goes back, with the folder you left selected,
`Alt+Up` goes up, and `Ctrl+L` types a path. `Ctrl+C`, `Ctrl+X` and `Ctrl+V` copy, cut and paste, also
between machines, `F2` renames and `Delete` moves to the trash. `F5`
or `Ctrl+R` lists the folder again, even with the keyboard elsewhere
in the window.
`Ctrl+F` filters the folder, `Ctrl+P` finds a file, `Ctrl+1` and
`Ctrl+2` switch between details and icons, and `Space` views a file.
`Ctrl+W` closes the pane. Keys with `Ctrl+Shift` stay kakel's, as
`Ctrl+Shift+D` to split and `Ctrl+Shift+C` to copy, except
`Ctrl+Shift+N`, a new folder. Copy path is on a file's menu.

Create zip… and Paste as zip… on the menus make a zip, and Extract…
unpacks a zip or a tar into a new folder beside it. Tick Protect with a
password to protect a new zip: what it holds is encrypted with AES-256,
which 7-Zip and WinZip open too, while the names of the files in it
stay readable. Extracting a zip a password protects, made by kakel,
7-Zip, WinZip or `zip -P`, asks for the password before it makes the
folder. The question offers your saved secrets, and can save the
password typed there once it has opened the zip, under the zip's name.
A secret with the zip's name opens it without asking.

The `+` at the end of the tabs opens a tab like the one in front: a
file manager at the same folder, or a terminal. Right-click it to pick
one.

`Space`, or View on a file's menu, shows the file over the window: a
picture large, code in its language's colours, Markdown rendered, and
anything else as its bytes in hex. `Space` or `Escape` closes it. The
preview beside the files colours code and renders Markdown too.
`Ctrl`+click a file's path in a terminal to view it the same way, at
the line a compiler named.
