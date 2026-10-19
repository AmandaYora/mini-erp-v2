package infrastructure

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// fakeSocket stands in for *whatsmeow.Client (T-02): it records calls and
// returns scripted errors, so the gateway logic runs without WhatsApp.
type fakeSocket struct {
	mu          sync.Mutex
	connected   bool
	connectErr  error
	logoutErr   error
	sendErr     error
	qrErr       error
	qrChan      chan whatsmeow.QRChannelItem
	connects    int
	disconnects int
	logouts     int
	qrCalls     int
	sentTo      []types.JID
	sentText    []string
}

func (f *fakeSocket) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

func (f *fakeSocket) Connect() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connects++
	if f.connectErr != nil {
		return f.connectErr
	}
	f.connected = true
	return nil
}

func (f *fakeSocket) Disconnect() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disconnects++
	f.connected = false
}

func (f *fakeSocket) Logout(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logouts++
	if f.logoutErr != nil {
		return f.logoutErr
	}
	f.connected = false
	return nil
}

func (f *fakeSocket) GetQRChannel(context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.qrCalls++
	if f.qrErr != nil {
		return nil, f.qrErr
	}
	if f.qrChan == nil {
		f.qrChan = make(chan whatsmeow.QRChannelItem, 8)
	}
	return f.qrChan, nil
}

func (f *fakeSocket) SendMessage(_ context.Context, to types.JID, m *waE2E.Message, _ ...whatsmeow.SendRequestExtra) (whatsmeow.SendResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return whatsmeow.SendResponse{}, f.sendErr
	}
	f.sentTo = append(f.sentTo, to)
	f.sentText = append(f.sentText, m.GetConversation())
	return whatsmeow.SendResponse{}, nil
}

// newTestGateway returns a gateway wired to the fake socket. paired controls
// whether the device already carries a JID (silent login) or not (needs QR).
func newTestGateway(paired bool) (*Gateway, *fakeSocket) {
	g := NewGateway(".", slog.New(slog.NewTextHandler(io.Discard, nil)))
	fs := &fakeSocket{}
	dev := &store.Device{}
	if paired {
		dev.ID = &types.JID{User: "6281234567890", Server: types.DefaultUserServer}
	}
	g.client, g.device = fs, dev
	return g, fs
}

func dm(sender, text string) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{
			Chat:   types.JID{User: sender, Server: types.DefaultUserServer},
			Sender: types.JID{User: sender, Server: types.DefaultUserServer},
		}},
		Message: &waE2E.Message{Conversation: proto.String(text)},
	}
}

type inbound struct{ phone, text string }

// capture installs OnMessage and returns a getter that waits briefly for the
// goroutine the gateway spawns; wantNone waits long enough to prove silence.
func capture(g *Gateway) (get func(wait time.Duration) []inbound) {
	var mu sync.Mutex
	var got []inbound
	g.OnMessage = func(phone, text string) {
		mu.Lock()
		got = append(got, inbound{phone, text})
		mu.Unlock()
	}
	return func(wait time.Duration) []inbound {
		deadline := time.Now().Add(wait)
		for time.Now().Before(deadline) {
			mu.Lock()
			n := len(got)
			mu.Unlock()
			if n > 0 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		return append([]inbound(nil), got...)
	}
}

func TestOnMessageDeliversTrimmedDM(t *testing.T) {
	g, _ := newTestGateway(true)
	get := capture(g)
	g.onMessage(dm("+62 812-3456-7890", "  stok hari ini  "))
	got := get(time.Second)
	if len(got) != 1 || got[0].phone != "6281234567890" || got[0].text != "stok hari ini" {
		t.Fatalf("got %+v, want one trimmed DM from digits-only phone", got)
	}
}

func TestOnMessageIgnoresNonDMTraffic(t *testing.T) {
	cases := map[string]func(ev *events.Message){
		"fromMe":         func(ev *events.Message) { ev.Info.IsFromMe = true },
		"group":          func(ev *events.Message) { ev.Info.IsGroup = true },
		"status":         func(ev *events.Message) { ev.Info.Chat = types.StatusBroadcastJID },
		"broadcast list": func(ev *events.Message) { ev.Info.Chat = types.JID{User: "123", Server: types.BroadcastServer} },
		"group server":   func(ev *events.Message) { ev.Info.Chat = types.JID{User: "123", Server: types.GroupServer} },
		"invalid phone":  func(ev *events.Message) { ev.Info.Sender = types.JID{User: "12", Server: types.DefaultUserServer} },
		"empty text":     func(ev *events.Message) { ev.Message = &waE2E.Message{Conversation: proto.String("   ")} },
		"nil message":    func(ev *events.Message) { ev.Message = nil },
		"sticker/other":  func(ev *events.Message) { ev.Message = &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}} },
		"audio":          func(ev *events.Message) { ev.Message = &waE2E.Message{AudioMessage: &waE2E.AudioMessage{}} },
		"location":       func(ev *events.Message) { ev.Message = &waE2E.Message{LocationMessage: &waE2E.LocationMessage{}} },
		"document without caption": func(ev *events.Message) {
			ev.Message = &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{FileName: proto.String("a.pdf")}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGateway(true)
			get := capture(g)
			ev := dm("6281234567890", "halo")
			mutate(ev)
			g.onMessage(ev)
			if got := get(80 * time.Millisecond); len(got) != 0 {
				t.Fatalf("%s must be ignored silently, got %+v", name, got)
			}
		})
	}
}

func TestOnMessageTextSources(t *testing.T) {
	cases := map[string]*waE2E.Message{
		"extended": {ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("balasan")}},
		"image":    {ImageMessage: &waE2E.ImageMessage{Caption: proto.String("keterangan foto")}},
		"video":    {VideoMessage: &waE2E.VideoMessage{Caption: proto.String("keterangan video")}},
	}
	want := map[string]string{"extended": "balasan", "image": "keterangan foto", "video": "keterangan video"}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGateway(true)
			get := capture(g)
			ev := dm("6281234567890", "")
			ev.Message = m
			g.onMessage(ev)
			got := get(time.Second)
			if len(got) != 1 || got[0].text != want[name] {
				t.Fatalf("%s: got %+v, want %q", name, got, want[name])
			}
		})
	}
}

func TestMessageTextPrefersConversation(t *testing.T) {
	ev := &events.Message{Message: &waE2E.Message{
		Conversation:        proto.String("utama"),
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("lain")},
		ImageMessage:        &waE2E.ImageMessage{Caption: proto.String("cap")},
	}}
	if got := messageText(ev); got != "utama" {
		t.Fatalf("messageText = %q, want conversation first", got)
	}
}

// lidDM builds a DM whose chat and sender are LID-addressed (N-44).
func lidDM(lid string, alt types.JID, text string) *events.Message {
	ev := dm("6281234567890", text)
	lidJID := types.JID{User: lid, Server: types.HiddenUserServer}
	ev.Info.Chat, ev.Info.Sender, ev.Info.SenderAlt = lidJID, lidJID, alt
	ev.Info.AddressingMode = types.AddressingModeLID
	return ev
}

// N-44: a LID-addressed DM is processed as the sender's phone number (PN)
// taken from SenderAlt, never as the LID itself.
func TestOnMessageLIDDMUsesSenderAltPhone(t *testing.T) {
	for name, lid := range map[string]string{"15-digit LID": "184467440737095", "short numeric LID": "12345678", "non-numeric LID": "abc"} {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGateway(true)
			get := capture(g)
			g.onMessage(lidDM(lid, types.JID{User: "6289876543210", Server: types.DefaultUserServer}, "cek"))
			got := get(time.Second)
			if len(got) != 1 || got[0].phone != "6289876543210" || got[0].text != "cek" {
				t.Fatalf("got %+v, want the PN from SenderAlt", got)
			}
		})
	}
}

// LID without a usable PN is dropped safely (no reply to an unknown number).
func TestOnMessageLIDWithoutPhoneIsDropped(t *testing.T) {
	cases := map[string]types.JID{
		"no alt":              {},
		"alt is also a LID":   {User: "99887766554433", Server: types.HiddenUserServer},
		"alt PN invalid":      {User: "12", Server: types.DefaultUserServer},
		"alt on other server": {User: "6289876543210", Server: types.HiddenUserServer},
	}
	for name, alt := range cases {
		t.Run(name, func(t *testing.T) {
			g, _ := newTestGateway(true)
			get := capture(g)
			g.onMessage(lidDM("184467440737095", alt, "cek"))
			if got := get(80 * time.Millisecond); len(got) != 0 {
				t.Fatalf("LID without PN must not be delivered (a LID is not a phone): %+v", got)
			}
		})
	}
}

// A PN sender wins even when the chat is LID-addressed; a PN chat with a LID
// sender falls back to SenderAlt; groups and status stay ignored in LID mode.
func TestOnMessageAddressingMixes(t *testing.T) {
	g, _ := newTestGateway(true)
	get := capture(g)
	ev := lidDM("184467440737095", types.JID{User: "6280000000000", Server: types.DefaultUserServer}, "x")
	ev.Info.Sender = types.JID{User: "6281234567890", Server: types.DefaultUserServer}
	g.onMessage(ev)
	if got := get(time.Second); len(got) != 1 || got[0].phone != "6281234567890" {
		t.Fatalf("PN sender must win: %+v", got)
	}

	g2, _ := newTestGateway(true)
	get2 := capture(g2)
	ev2 := dm("6281234567890", "y")
	ev2.Info.Sender = types.JID{User: "184467440737095", Server: types.HiddenUserServer}
	ev2.Info.SenderAlt = types.JID{User: "6289876543210", Server: types.DefaultUserServer}
	g2.onMessage(ev2)
	if got := get2(time.Second); len(got) != 1 || got[0].phone != "6289876543210" {
		t.Fatalf("PN chat + LID sender: %+v", got)
	}

	for name, mutate := range map[string]func(*events.Message){
		"lid group": func(e *events.Message) {
			e.Info.IsGroup = true
			e.Info.Chat = types.JID{User: "1", Server: types.GroupServer}
		},
		"lid status": func(e *events.Message) { e.Info.Chat = types.StatusBroadcastJID },
		"lid fromMe": func(e *events.Message) { e.Info.IsFromMe = true },
		"hosted lid": func(e *events.Message) { e.Info.Chat = types.JID{User: "1", Server: types.HostedLIDServer} },
		"bot chat":   func(e *events.Message) { e.Info.Chat = types.JID{User: "1", Server: types.BotServer} },
	} {
		g3, _ := newTestGateway(true)
		get3 := capture(g3)
		e := lidDM("184467440737095", types.JID{User: "6289876543210", Server: types.DefaultUserServer}, "z")
		mutate(e)
		g3.onMessage(e)
		if got := get3(60 * time.Millisecond); len(got) != 0 {
			t.Errorf("%s must be ignored: %+v", name, got)
		}
	}
}

func TestOnMessageWithoutHandlerAndPanickingHandler(t *testing.T) {
	g, _ := newTestGateway(true)
	g.onMessage(dm("6281234567890", "tanpa handler")) // must not panic

	done := make(chan struct{})
	g.OnMessage = func(string, string) {
		defer close(done)
		panic("boom")
	}
	g.onMessage(dm("6281234567890", "halo"))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler was not invoked")
	}
	time.Sleep(20 * time.Millisecond) // recover() ran; test process still alive
}

func TestOnEventLifecycle(t *testing.T) {
	g, _ := newTestGateway(true)
	g.qr, g.lastErr = "old", "old"
	g.onEvent(&events.Connected{})
	if g.QR() != "" || g.LastError() != "" {
		t.Fatalf("Connected must clear qr/err: %q %q", g.QR(), g.LastError())
	}
	g.onEvent(&events.Disconnected{})
	if g.LastError() != "koneksi terputus" {
		t.Fatalf("Disconnected err = %q", g.LastError())
	}
	g.onEvent(&events.QR{Codes: []string{"c1", "c2"}})
	if g.QR() != "c1" {
		t.Fatalf("QR event keeps first code, got %q", g.QR())
	}
	g.onEvent("unrelated event type") // ignored, no panic
}

func TestWatchQRTracksCodesSuccessAndTimeout(t *testing.T) {
	g, _ := newTestGateway(false)
	g.qrWatch = true
	ch := make(chan whatsmeow.QRChannelItem, 4)
	done := make(chan struct{})
	go func() { g.watchQR(ch); close(done) }()

	ch <- whatsmeow.QRChannelItem{Event: "code", Code: "A"}
	waitFor(t, func() bool { return g.QR() == "A" })
	ch <- whatsmeow.QRChannelItem{Event: "code", Code: "B"}
	waitFor(t, func() bool { return g.QR() == "B" })
	ch <- whatsmeow.QRChannelItem{Event: "err-client-outdated", Error: errors.New("klien usang")}
	waitFor(t, func() bool { return g.LastError() == "klien usang" && g.QR() == "" })
	ch <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelSuccess.Event}
	waitFor(t, func() bool { return g.LastError() == "" })
	close(ch)
	<-done
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.qr != "" || g.qrWatch {
		t.Fatalf("watcher must release on close: qr=%q watch=%v", g.qr, g.qrWatch)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestQRDataURL(t *testing.T) {
	g, _ := newTestGateway(false)
	if g.QRDataURL() != "" {
		t.Fatal("no pending QR must yield empty data URL")
	}
	g.qr = "2@abc,def,ghi"
	if u := g.QRDataURL(); !strings.HasPrefix(u, "data:image/png;base64,") || len(u) < 100 {
		t.Fatalf("data url = %.40q", u)
	}
}

func TestSendText(t *testing.T) {
	g, fs := newTestGateway(true)
	ctx := context.Background()

	if err := g.SendText(ctx, "6281234567890", "halo"); err == nil || !strings.Contains(err.Error(), "belum terhubung") {
		t.Fatalf("offline send err = %v", err)
	}
	fs.connected = true
	if err := g.SendText(ctx, "6281234567890", "halo"); err != nil {
		t.Fatal(err)
	}
	if len(fs.sentTo) != 1 || fs.sentTo[0].User != "6281234567890" || fs.sentTo[0].Server != types.DefaultUserServer || fs.sentText[0] != "halo" {
		t.Fatalf("sent = %+v %+v", fs.sentTo, fs.sentText)
	}
	fs.sendErr = errors.New("jaringan putus")
	if err := g.SendText(ctx, "6281234567890", "x"); err == nil || err.Error() != "jaringan putus" {
		t.Fatalf("send error must propagate, got %v", err)
	}
	if err := g.SendText(ctx, "bukan nomor@@", "x"); err == nil {
		// ParseJID accepts odd users; the contract is only "no panic".
		t.Log("ParseJID accepted odd phone")
	}

	g.Close()
	if err := g.SendText(ctx, "6281234567890", "x"); err == nil || !strings.Contains(err.Error(), "ditutup") {
		t.Fatalf("send after Close err = %v", err)
	}
}

func TestConnectPairedReconnectsSilently(t *testing.T) {
	g, fs := newTestGateway(true)
	ctx := context.Background()
	if !g.Paired(ctx) {
		t.Fatal("device with JID is paired")
	}
	if err := g.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if fs.connects != 1 || fs.qrCalls != 0 {
		t.Fatalf("paired connect: connects=%d qrCalls=%d (no QR expected)", fs.connects, fs.qrCalls)
	}
	if !g.Connected() || g.Phone() != "6281234567890" {
		t.Fatalf("connected=%v phone=%q", g.Connected(), g.Phone())
	}
	// Idempotent while connected.
	if err := g.Connect(ctx); err != nil || fs.connects != 1 {
		t.Fatalf("second connect must be a no-op: err=%v connects=%d", err, fs.connects)
	}
	// Disconnect keeps the session; reconnect needs no QR.
	g.Disconnect()
	if g.Connected() || fs.disconnects != 1 {
		t.Fatalf("after disconnect connected=%v disconnects=%d", g.Connected(), fs.disconnects)
	}
	if err := g.Connect(ctx); err != nil || fs.connects != 2 || fs.qrCalls != 0 {
		t.Fatalf("reconnect: err=%v connects=%d qr=%d", err, fs.connects, fs.qrCalls)
	}
	// A Disconnected event surfaces the failure, a Connected event clears it.
	g.onEvent(&events.Disconnected{})
	if g.LastError() == "" {
		t.Fatal("disconnect event must surface an error")
	}
	g.onEvent(&events.Connected{})
	if g.LastError() != "" {
		t.Fatal("connected event must clear the error")
	}
}

func TestConnectFailureIsRecorded(t *testing.T) {
	g, fs := newTestGateway(true)
	fs.connectErr = errors.New("tidak bisa menjangkau server")
	if err := g.Connect(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if g.LastError() != "tidak bisa menjangkau server" || g.Connected() {
		t.Fatalf("lastErr=%q connected=%v", g.LastError(), g.Connected())
	}
}

func TestConnectUnpairedStartsQRWatcherOnce(t *testing.T) {
	g, fs := newTestGateway(false)
	ctx := context.Background()
	if g.Paired(ctx) {
		t.Fatal("device without JID is not paired")
	}
	if g.Phone() != "" {
		t.Fatalf("unpaired phone must be empty, got %q", g.Phone())
	}
	if err := g.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if fs.qrCalls != 1 {
		t.Fatalf("qrCalls = %d", fs.qrCalls)
	}
	fs.qrChan <- whatsmeow.QRChannelItem{Event: "code", Code: "PAIR-1"}
	waitFor(t, func() bool { return g.QR() == "PAIR-1" })

	// While pairing is in flight a second Connect (socket dropped) must not
	// open a second QR channel.
	fs.connected = false
	if err := g.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if fs.qrCalls != 1 {
		t.Fatalf("second QR channel opened: %d", fs.qrCalls)
	}
	close(fs.qrChan) // pairing timed out
	waitFor(t, func() bool { g.mu.Lock(); defer g.mu.Unlock(); return !g.qrWatch && g.qr == "" })
}

func TestConnectQRChannelErrorIsRecorded(t *testing.T) {
	g, fs := newTestGateway(false)
	fs.qrErr = errors.New("sudah terhubung")
	if err := g.Connect(context.Background()); err == nil || g.LastError() != "sudah terhubung" {
		t.Fatalf("err=%v lastErr=%q", err, g.LastError())
	}
	if fs.connects != 0 {
		t.Fatal("must not connect when the QR channel failed")
	}
}

func TestResetBehaviour(t *testing.T) {
	ctx := context.Background()

	// Connected: logout clears the pending QR.
	g, fs := newTestGateway(true)
	fs.connected, g.qr = true, "stale"
	if err := g.Reset(ctx); err != nil || fs.logouts != 1 || g.QR() != "" {
		t.Fatalf("reset: err=%v logouts=%d qr=%q", err, fs.logouts, g.QR())
	}

	// Connected but logout fails: error surfaces.
	g, fs = newTestGateway(true)
	fs.connected, fs.logoutErr = true, errors.New("logout gagal")
	if err := g.Reset(ctx); err == nil {
		t.Fatal("connected logout failure must surface")
	}

	// Offline + paired + transport error: surfaces (keys must not be assumed gone).
	g, fs = newTestGateway(true)
	fs.logoutErr = errors.New("offline")
	if err := g.Reset(ctx); err == nil {
		t.Fatal("offline logout of a paired device must surface the error")
	}

	// Offline + never paired: the error is ignored.
	g, fs = newTestGateway(false)
	fs.logoutErr = errors.New("not logged in")
	if err := g.Reset(ctx); err != nil {
		t.Fatalf("unpaired reset must succeed, got %v", err)
	}
}

func TestCloseIsIdempotentAndBlocksReuse(t *testing.T) {
	g, fs := newTestGateway(true)
	fs.connected = true
	g.Close()
	g.Close()
	if fs.disconnects != 1 {
		t.Fatalf("disconnects = %d, want exactly 1", fs.disconnects)
	}
	if g.Connected() || g.Phone() != "" {
		t.Fatal("closed gateway must report disconnected and no phone")
	}
	if err := g.Connect(context.Background()); err == nil || !strings.Contains(err.Error(), "ditutup") {
		t.Fatalf("connect after close err = %v", err)
	}
	if g.Paired(context.Background()) {
		t.Fatal("closed gateway is never paired")
	}

	// Close before any connection must not panic.
	NewGateway(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil))).Close()
}

func TestDigitsAndValidPhone(t *testing.T) {
	if got := digits("+62 812-3456 (7890)"); got != "6281234567890" {
		t.Fatalf("digits = %q", got)
	}
	for in, want := range map[string]bool{
		"": false, "1234567": false, "12345678": true, "123456789012345": true,
		"1234567890123456": false, "62812abc78": false,
	} {
		if got := validPhone(in); got != want {
			t.Errorf("validPhone(%q) = %v, want %v", in, got, want)
		}
	}
}
