// Package winattrs carries the Windows attributes that say how a cloud
// provider such as OneDrive keeps a file, over SFTP, from a kakel window
// serving its files to the kakel connected to it.
//
// Plain SFTP has no word for them, but its replies may carry extended
// attributes. Proxy stands between the SFTP server and the connection
// and adds the attributes to each item of a folder listing and of a
// stat, under Name, where any is set. The connected kakel finds them in
// the replies as they come, so the status of a folder's files costs no
// request of its own. An ordinary SFTP server sends none.
//
// The proxy also tells the cloud provider which items to keep on the
// device, as Explorer's Free up space and Always keep on this device do,
// where the connected kakel sets KeepName on an item; it says it can in
// the server's version reply, as an extension of that name.
package winattrs

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Name is the extended attribute the attributes go under, as a number.
const Name = "kakel-winattrs@kakel"

// The attributes carried: those a cloud provider sets.
const (
	Offline            = 0x1000
	Pinned             = 0x80000
	Unpinned           = 0x100000
	RecallOnOpen       = 0x40000
	RecallOnDataAccess = 0x400000
	// InCloud is no attribute of Windows' own, which leave the top bit
	// alone: it says the item lies in a cloud provider's folder, as a
	// file there kept on this device has no attribute that says so.
	InCloud = 0x80000000
	cloud   = Offline | Pinned | Unpinned | RecallOnOpen | RecallOnDataAccess | InCloud
)

// KeepName is the extended attribute the connected kakel sets on an item
// to have the cloud provider keep it, and all a folder holds, on the
// device, with "1", or free their space, with "0". The proxy answers it
// itself, and names it among the server's extensions where it can.
const KeepName = "kakel-keep@kakel"

// OnlineOnly reports whether attrs say the file's contents are kept
// online only, so reading it downloads it.
func OnlineOnly(attrs uint32) bool {
	return attrs&(Offline|RecallOnOpen|RecallOnDataAccess) != 0
}

// Parse reads the attributes from an extended attribute's value.
func Parse(v string) (uint32, bool) {
	n, err := strconv.ParseUint(v, 10, 32)
	return uint32(n), err == nil
}

// Lookup returns the attributes of the item at the SFTP path p, as the
// server reads it, following a link at its end with follow, or false
// where it has none to say; nil where the system keeps none. home is
// the folder a relative path starts in. It may be called from several
// goroutines at once.
var Lookup func(home, p string, follow bool) (uint32, bool)

// Space returns how many bytes are free to the user and in all on the
// volume that holds the SFTP path p, or false where it can't say; nil
// where the proxy leaves the question to the server. home is the folder
// a relative path starts in.
var Space func(home, p string) (free, total uint64, ok bool)

// Keep has the cloud provider keep the item at the SFTP path p, and all
// a folder holds, on the device with keep, or free their space without,
// until ctx ends; nil where the system has no cloud provider to tell.
// home is the folder a relative path starts in.
var Keep func(ctx context.Context, home, p string, keep bool) error

// statvfs is OpenSSH's extension asking for a volume's space, which Go's
// SFTP server can't answer on Windows.
const statvfs = "statvfs@openssh.com"

// The SFTP packets the proxy reads.
const (
	fxpExtended      = 200
	fxpExtendedReply = 201
	fxpInit          = 1
	fxpVersion       = 2
	fxpSetstat       = 9
	fxpLstat         = 7
	fxpOpendir       = 11
	fxpReaddir       = 12
	fxpStat          = 17
	fxpClose         = 4
	fxpStatus        = 101
	fxpHandle        = 102
	fxpName          = 104
	fxpAttrs         = 105
	attrSize         = 0x1
	attrUIDGID       = 0x2
	attrPerms        = 0x4
	attrTimes        = 0x8
	attrExtends      = 0x80000000
	// The codes of a status reply.
	fxOK          = 0
	fxFailure     = 4
	fxUnsupported = 8
	// maxPacket is the longest packet read, as pkg/sftp takes.
	maxPacket = 1 << 18
)

// Proxy returns the end of conn an SFTP server serves on: what it reads
// comes from conn, and what it writes goes to conn, with the attributes
// lookup gives added to its listings and stats. home is the folder the
// server starts relative paths in. Closing what it returns closes conn.
//
// A reply that needs attributes looked up waits for them on a goroutine
// of its own, so a slow disk holds up no other reply: the client matches
// replies to requests by their IDs, in whatever order they come.
//
// With space, it answers OpenSSH's statvfs itself, from it, in place of
// the server. With keep, it sets KeepName itself, by it, and names it
// among the server's extensions.
func Proxy(conn io.ReadWriteCloser, home string, lookup func(home, p string, follow bool) (uint32, bool),
	space func(home, p string) (free, total uint64, ok bool),
	keep func(ctx context.Context, home, p string, keep bool) error) io.ReadWriteCloser {
	toServer, fromConn := io.Pipe()
	toConn, fromServer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	p := &proxy{home: home, lookup: lookup, space: space, keep: keep, ctx: ctx, conn: conn, dirs: map[string]string{},
		opening: map[uint32]string{}, reading: map[uint32]string{}, stating: map[uint32]stat{}, busy: make(chan struct{}, 4),
		keeping: make(chan struct{}, mostKeeping)}
	go func() {
		err := p.requests(conn, fromConn)
		// What it was asked to keep or free stops with the connection.
		cancel()
		_ = fromConn.CloseWithError(err)
	}()
	go func() {
		err := p.replies(toConn, conn)
		_ = toConn.CloseWithError(err)
	}()
	return &end{Reader: toServer, Writer: fromServer, close: func() error {
		cancel()
		_ = toServer.Close()
		_ = fromServer.Close()
		return conn.Close()
	}}
}

type end struct {
	io.Reader
	io.Writer
	close func() error
}

func (e *end) Close() error { return e.close() }

// proxy follows which folder each handle and request is about.
type proxy struct {
	home   string
	lookup func(home, p string, follow bool) (uint32, bool)
	space  func(home, p string) (free, total uint64, ok bool)
	keep   func(ctx context.Context, home, p string, keep bool) error
	// ctx ends with the connection, and with it what keep does; keeping
	// bounds how many items are kept or freed at once.
	ctx     context.Context
	keeping chan struct{}
	// conn is where the proxy answers what it answers itself.
	conn io.Writer
	mu   sync.Mutex
	// dirs are the folders open, by handle; opening, reading and
	// stating the requests waiting for their reply, by ID.
	dirs             map[string]string
	opening, reading map[uint32]string
	stating          map[uint32]stat
	// wmu keeps the packets written to the connection whole, and busy
	// bounds the replies waiting for their attributes at once.
	wmu  sync.Mutex
	busy chan struct{}
}

// stat is an item stated, and whether a link at its end is followed.
type stat struct {
	path   string
	follow bool
}

// requests passes the packets from the connection to the server, noting
// the folders opened and read and the items stated.
func (p *proxy) requests(from io.Reader, to io.Writer) error {
	for {
		pkt, err := readPacket(from)
		if err != nil {
			return err
		}
		if p.keeps(pkt) {
			continue
		}
		if reply, ok := p.answer(pkt); ok {
			if err := p.reply(reply); err != nil {
				return err
			}
			continue
		}
		p.note(pkt)
		if _, err := to.Write(pkt); err != nil {
			return err
		}
	}
}

// replies passes the packets from the server to the connection, adding
// the attributes.
func (p *proxy) replies(from io.Reader, to io.Writer) error {
	var wg sync.WaitGroup
	defer wg.Wait()
	write := func(pkt []byte) error {
		p.wmu.Lock()
		defer p.wmu.Unlock()
		_, err := to.Write(pkt)
		return err
	}
	for {
		pkt, err := readPacket(from)
		if err != nil {
			return err
		}
		if pkt[4] == fxpVersion && p.keep != nil {
			pkt = appendExtension(pkt, KeepName, "1")
		}
		rewrite := p.claim(pkt)
		if rewrite == nil {
			if err := write(pkt); err != nil {
				return err
			}
			continue
		}
		p.busy <- struct{}{}
		wg.Go(func() {
			defer func() { <-p.busy }()
			if out, ok := rewrite(); ok {
				pkt = out
			}
			// A connection gone fails the next reply written above.
			_ = write(pkt)
		})
	}
}

// readPacket reads one packet, its length with it.
func readPacket(r io.Reader) ([]byte, error) {
	var n [4]byte
	if _, err := io.ReadFull(r, n[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(n[:])
	if size == 0 || size > maxPacket {
		return nil, errors.New("winattrs: a packet of " + strconv.FormatUint(uint64(size), 10) + " bytes")
	}
	pkt := make([]byte, 4+size)
	copy(pkt, n[:])
	if _, err := io.ReadFull(r, pkt[4:]); err != nil {
		return nil, err
	}
	return pkt, nil
}

// answer is the reply to a request the proxy answers itself, a volume's
// space, and false for any it passes on.
func (p *proxy) answer(pkt []byte) ([]byte, bool) {
	b := pkt[4:]
	if p.space == nil || len(b) < 5 || b[0] != fxpExtended {
		return nil, false
	}
	id := binary.BigEndian.Uint32(b[1:5])
	name, rest, ok := str(b[5:])
	if !ok || name != statvfs {
		return nil, false
	}
	at, _, ok := str(rest)
	if !ok {
		return nil, false
	}
	free, total, ok := p.space(p.home, at)
	if !ok {
		// The server says it can't, as it would.
		return nil, false
	}
	// A block of one byte: the counts are the bytes.
	out := []byte{fxpExtendedReply}
	out = binary.BigEndian.AppendUint32(out, id)
	for _, v := range []uint64{1, 1, total, free, free, 0, 0, 0, 0, 0, 255} {
		out = binary.BigEndian.AppendUint64(out, v)
	}
	return packet(out), true
}

// mostKeeping is how many items the proxy keeps or frees at once: each
// walks all a folder holds. One more is refused.
const mostKeeping = 2

// reply writes a reply the proxy makes itself to the connection.
func (p *proxy) reply(pkt []byte) error {
	p.wmu.Lock()
	defer p.wmu.Unlock()
	_, err := p.conn.Write(pkt)
	return err
}

// keeps takes a request that sets KeepName on an item, and nothing else:
// it has the cloud provider keep the item on the device, or free it, on a
// goroutine of its own, as that may take a while, and answers once done.
// It reports whether the request was one.
func (p *proxy) keeps(pkt []byte) bool {
	b := pkt[4:]
	if p.keep == nil || len(b) < 5 || b[0] != fxpSetstat {
		return false
	}
	id := binary.BigEndian.Uint32(b[1:5])
	at, rest, ok := str(b[5:])
	if !ok || len(rest) < 8 || binary.BigEndian.Uint32(rest) != attrExtends {
		return false
	}
	count := binary.BigEndian.Uint32(rest[4:])
	rest = rest[8:]
	value := ""
	for range count {
		k, after, ok := str(rest)
		if !ok {
			return false
		}
		v, after2, ok := str(after)
		if !ok {
			return false
		}
		if k == KeepName {
			value = v
		}
		rest = after2
	}
	if value != "0" && value != "1" || count != 1 {
		return false
	}
	select {
	case p.keeping <- struct{}{}:
	default:
		go func() { _ = p.reply(status(id, fxFailure, "busy with others; try again once they are done")) }()
		return true
	}
	go func() {
		defer func() { <-p.keeping }()
		err := p.keep(p.ctx, p.home, at, value == "1")
		code, msg := uint32(fxOK), ""
		switch {
		case errors.Is(err, errors.ErrUnsupported):
			code, msg = fxUnsupported, err.Error()
		case err != nil:
			code, msg = fxFailure, err.Error()
		}
		// A connection gone has no one to tell.
		_ = p.reply(status(id, code, msg))
	}()
	return true
}

// status is a status reply to request id.
func status(id, code uint32, msg string) []byte {
	out := []byte{fxpStatus}
	out = binary.BigEndian.AppendUint32(out, id)
	out = binary.BigEndian.AppendUint32(out, code)
	out = appendStr(out, msg)
	out = appendStr(out, "")
	return packet(out)
}

// appendExtension is the server's version reply pkt naming the extension
// name too, with data.
func appendExtension(pkt []byte, name, data string) []byte {
	out := slices.Clone(pkt[4:])
	out = appendStr(out, name)
	out = appendStr(out, data)
	return packet(out)
}

// note notes what a request is about.
func (p *proxy) note(pkt []byte) {
	b := pkt[4:]
	kind := b[0]
	if kind == fxpInit || len(b) < 5 {
		return
	}
	id := binary.BigEndian.Uint32(b[1:5])
	s, _, ok := str(b[5:])
	if !ok {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	switch kind {
	case fxpOpendir:
		p.opening[id] = s
	case fxpReaddir:
		if dir, ok := p.dirs[s]; ok {
			p.reading[id] = dir
		}
	case fxpStat, fxpLstat:
		p.stating[id] = stat{path: s, follow: kind == fxpStat}
	case fxpClose:
		delete(p.dirs, s)
	}
}

// claim notes what a reply says of the requests it answers, a folder's
// handle before the client can use it, and returns what adds the
// attributes to a reply to a listing or a stat; nil for any other.
func (p *proxy) claim(pkt []byte) func() ([]byte, bool) {
	b := pkt[4:]
	if len(b) < 5 {
		return nil
	}
	kind, id := b[0], binary.BigEndian.Uint32(b[1:5])
	p.mu.Lock()
	opened, isOpen := p.opening[id]
	dir, isRead := p.reading[id]
	item, isStat := p.stating[id]
	delete(p.opening, id)
	delete(p.reading, id)
	delete(p.stating, id)
	if kind == fxpHandle && isOpen {
		if h, _, ok := str(b[5:]); ok {
			p.dirs[h] = opened
		}
	}
	p.mu.Unlock()
	switch {
	case kind == fxpName && isRead:
		return func() ([]byte, bool) { return p.listing(b, dir) }
	case kind == fxpAttrs && isStat:
		return func() ([]byte, bool) {
			v, has := p.lookup(p.home, item.path, item.follow)
			attrs, rest, ok := with(b[5:], v, has)
			if !ok || len(rest) != 0 {
				return nil, false
			}
			return packet(append(slices.Clone(b[:5]), attrs...)), true
		}
	}
	return nil
}

// listing adds the attributes to each item of a folder's listing, all
// looked up at once.
func (p *proxy) listing(b []byte, dir string) ([]byte, bool) {
	if len(b) < 9 {
		return nil, false
	}
	n := binary.BigEndian.Uint32(b[5:9])
	type item struct {
		name, head, attrs []byte
		v                 uint32
		has               bool
	}
	var items []item
	rest := b[9:]
	for range n {
		name, after, ok := str(rest)
		if !ok {
			return nil, false
		}
		_, after2, ok := str(after)
		if !ok {
			return nil, false
		}
		_, left, ok := with(after2, 0, false)
		if !ok {
			return nil, false
		}
		items = append(items, item{name: []byte(name), head: rest[:len(rest)-len(after2)], attrs: after2[:len(after2)-len(left)]})
		rest = left
	}
	if len(rest) != 0 {
		return nil, false
	}
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(8, len(items)) {
		wg.Go(func() {
			for i := range next {
				items[i].v, items[i].has = p.lookup(p.home, path.Join(dir, string(items[i].name)), false)
			}
		})
	}
	for i := range items {
		next <- i
	}
	close(next)
	wg.Wait()
	out := slices.Clone(b[:9])
	for _, it := range items {
		out = append(out, it.head...)
		attrs, _, _ := with(it.attrs, it.v, it.has)
		out = append(out, attrs...)
	}
	return packet(out), true
}

// with is the attributes at the start of b, with v added where has says
// the item has any of a cloud provider's, and what follows them.
func with(b []byte, v uint32, has bool) (attrs, rest []byte, ok bool) {
	if len(b) < 4 {
		return nil, nil, false
	}
	flags := binary.BigEndian.Uint32(b)
	at := 4
	if flags&attrSize != 0 {
		at += 8
	}
	if flags&attrUIDGID != 0 {
		at += 8
	}
	if flags&attrPerms != 0 {
		at += 4
	}
	if flags&attrTimes != 0 {
		at += 8
	}
	if at > len(b) {
		return nil, nil, false
	}
	var ext [][2]string
	end := at
	if flags&attrExtends != 0 {
		if end+4 > len(b) {
			return nil, nil, false
		}
		count := binary.BigEndian.Uint32(b[end:])
		end += 4
		for range count {
			k, after, ok := str(b[end:])
			if !ok {
				return nil, nil, false
			}
			v, after2, ok := str(after)
			if !ok {
				return nil, nil, false
			}
			ext = append(ext, [2]string{k, v})
			end = len(b) - len(after2)
		}
	}
	whole, rest := b[:end], b[end:]
	if !has || v&cloud == 0 {
		return whole, rest, true
	}
	ext = append(ext, [2]string{Name, strconv.FormatUint(uint64(v&cloud), 10)})
	out := binary.BigEndian.AppendUint32(nil, flags|attrExtends)
	out = append(out, b[4:at]...)
	out = binary.BigEndian.AppendUint32(out, uint32(len(ext)))
	for _, kv := range ext {
		out = appendStr(out, kv[0])
		out = appendStr(out, kv[1])
	}
	return out, rest, true
}

// str reads an SFTP string.
func str(b []byte) (s string, rest []byte, ok bool) {
	if len(b) < 4 {
		return "", nil, false
	}
	n := binary.BigEndian.Uint32(b)
	if uint64(n) > uint64(len(b)-4) {
		return "", nil, false
	}
	return string(b[4 : 4+n]), b[4+n:], true
}

func appendStr(b []byte, s string) []byte {
	b = binary.BigEndian.AppendUint32(b, uint32(len(s)))
	return append(b, s...)
}

// packet puts the length before b.
func packet(b []byte) []byte {
	return append(binary.BigEndian.AppendUint32(nil, uint32(len(b))), b...)
}

// Resolve is the SFTP path p as a path of the server's, with slashes,
// relative ones from home, written with slashes too: on Windows,
// /C:/Users is C:/Users.
func Resolve(home, p string) string {
	if !strings.HasPrefix(p, "/") {
		p = path.Join(home, p)
	}
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	if len(p) == 2 && p[1] == ':' {
		// A drive by itself is its top folder, not the folder the
		// process is in on it.
		p += "/"
	}
	return p
}
