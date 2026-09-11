package infrastructure

import (
	"context"
	"database/sql"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "modernc.org/sqlite"
)

// Gateway wraps one whatsmeow client (single-tenant: one company number).
// The session cryptostore is a SQLite file beside uploads (current whatsmeow
// sqlstore supports SQLite/Postgres only — no MySQL). App data stays in
// MySQL; only WA keys live here. modernc driver = pure Go, no cgo.
// All methods are safe for concurrent use.
type Gateway struct {
	sessionFile string
	log         *slog.Logger

	mu        sync.Mutex
	container *sqlstore.Container
	device    *store.Device
	client    *whatsmeow.Client
	qr        string
	qrWatch   bool
	lastErr   string

	// OnMessage receives normalized inbound DMs (digits phone, trimmed text).
	OnMessage func(phone, text string)
}

// NewGateway builds the wrapper. Nothing connects until Connect/Boot.
func NewGateway(storageDir string, log *slog.Logger) *Gateway {
	return &Gateway{sessionFile: filepath.Join(storageDir, "whatsapp_session.db"), log: log}
}

// ensureClient lazily opens the session store and client (idempotent).
func (g *Gateway) ensureClient(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.client != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(g.sessionFile), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+g.sessionFile)
	if err != nil {
		return err
	}
	container := sqlstore.NewWithDB(db, "sqlite3", waLog.Noop)
	if err := container.Upgrade(ctx); err != nil {
		_ = db.Close()
		return err
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return err
	}
	if device == nil {
		device = container.NewDevice()
	}
	client := whatsmeow.NewClient(device, waLog.Noop)
	client.AddEventHandler(g.onEvent)
	g.container, g.device, g.client = container, device, client
	return nil
}

// Connected reports live socket state.
func (g *Gateway) Connected() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.client != nil && g.client.IsConnected()
}

// Phone returns the linked company number digits, or "" when unpaired.
func (g *Gateway) Phone() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.device == nil || g.device.ID == nil {
		return ""
	}
	return digits(g.device.ID.User)
}

// Paired reports whether a session was ever linked (silent reconnect
// possible without QR). Initializes the store on first call.
func (g *Gateway) Paired(ctx context.Context) bool {
	if err := g.ensureClient(ctx); err != nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.device != nil && g.device.ID != nil
}

// QR returns the latest pairing code, or "" when none is pending.
func (g *Gateway) QR() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.qr
}

// LastError returns the last gateway failure text.
func (g *Gateway) LastError() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastErr
}

// QRDataURL renders the pending QR as a PNG data URL for <img>.
func (g *Gateway) QRDataURL() string {
	code := g.QR()
	if code == "" {
		return ""
	}
	png, err := qrcode.Encode(code, qrcode.Medium, 220)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// Connect starts the socket. Unpaired devices emit QR codes (polled via
// Status); paired devices log in silently. Idempotent while connected.
func (g *Gateway) Connect(ctx context.Context) error {
	if err := g.ensureClient(ctx); err != nil {
		g.setError(err.Error())
		return err
	}
	g.mu.Lock()
	if g.client.IsConnected() {
		g.mu.Unlock()
		return nil
	}
	var qrChan <-chan whatsmeow.QRChannelItem
	if g.device.ID == nil && !g.qrWatch {
		ch, err := g.client.GetQRChannel(ctx)
		if err != nil {
			g.mu.Unlock()
			g.setError(err.Error())
			return err
		}
		qrChan, g.qrWatch = ch, true
	}
	g.mu.Unlock()

	if qrChan != nil {
		go g.watchQR(qrChan)
	}
	if err := g.client.Connect(); err != nil {
		g.setError(err.Error())
		return err
	}
	return nil
}

// watchQR tracks pairing codes until success/timeout, then releases.
func (g *Gateway) watchQR(ch <-chan whatsmeow.QRChannelItem) {
	defer func() {
		g.mu.Lock()
		g.qr, g.qrWatch = "", false
		g.mu.Unlock()
	}()
	for item := range ch {
		g.mu.Lock()
		switch {
		case item.Event == "code":
			g.qr, g.lastErr = item.Code, ""
		case item.Error != nil:
			g.qr, g.lastErr = "", item.Error.Error()
		case item.Event == whatsmeow.QRChannelSuccess.Event:
			g.qr, g.lastErr = "", ""
		}
		g.mu.Unlock()
	}
}

// Disconnect drops the socket but keeps the session: reconnect needs no QR.
func (g *Gateway) Disconnect() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.client != nil {
		g.client.Disconnect()
	}
	g.qr = ""
}

// Reset wipes the session (logout): the next connect needs a fresh QR scan.
func (g *Gateway) Reset(ctx context.Context) error {
	if err := g.ensureClient(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	client := g.client
	g.mu.Unlock()
	if client.IsConnected() {
		if err := client.Logout(ctx); err != nil {
			return err
		}
	} else if err := client.Logout(ctx); err != nil {
		// Offline logout still clears local keys; ignore transport errors
		// only when the device was never paired.
		g.mu.Lock()
		paired := g.device != nil && g.device.ID != nil
		g.mu.Unlock()
		if paired {
			return err
		}
	}
	g.mu.Lock()
	g.qr = ""
	g.mu.Unlock()
	return nil
}

// SendText delivers one DM. The caller owns retry policy (inbound path is
// best-effort: a failed send marks the message failed, never crashes).
func (g *Gateway) SendText(ctx context.Context, phone, text string) error {
	if err := g.ensureClient(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	client := g.client
	g.mu.Unlock()
	if client == nil || !client.IsConnected() {
		return errNotConnected()
	}
	jid, err := types.ParseJID(phone + "@s.whatsapp.net")
	if err != nil {
		return err
	}
	_, err = client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(text),
	})
	return err
}

func (g *Gateway) setError(s string) {
	g.mu.Lock()
	g.lastErr = s
	g.mu.Unlock()
	g.log.Error("whatsapp gateway", "error", s)
}

// onEvent routes socket events: pairing progress, failures, inbound DMs.
func (g *Gateway) onEvent(raw any) {
	switch ev := raw.(type) {
	case *events.Connected:
		g.mu.Lock()
		g.qr, g.lastErr = "", ""
		g.mu.Unlock()
		g.log.Info("whatsapp connected")
	case *events.Disconnected:
		g.setError("koneksi terputus")
	case *events.QR:
		// Backup path (QR channel is primary); keep latest code too.
		for _, c := range ev.Codes {
			g.mu.Lock()
			g.qr = c
			g.mu.Unlock()
			break
		}
	case *events.Message:
		g.onMessage(ev)
	}
}

// onMessage filters one inbound event down to a DM worth answering:
// human text in a 1:1 chat, not from us. Groups, broadcasts, statuses,
// stickers, audio, locations and documents are ignored silently (legacy).
func (g *Gateway) onMessage(ev *events.Message) {
	if ev.Info.IsFromMe || ev.Info.IsGroup {
		return
	}
	if ev.Info.Chat.Server != types.DefaultUserServer {
		return
	}
	phone := digits(ev.Info.Sender.User)
	if !validPhone(phone) {
		phone = digits(ev.Info.SenderAlt.User)
		if !validPhone(phone) {
			return
		}
	}
	text := messageText(ev)
	if strings.TrimSpace(text) == "" {
		return
	}
	if g.OnMessage == nil {
		return
	}
	text = strings.TrimSpace(text)
	go func() {
		defer func() {
			_ = recover()
		}()
		g.OnMessage(phone, text)
	}()
}

// messageText extracts conversation / extended text / image / video caption.
func messageText(ev *events.Message) string {
	m := ev.Message
	if m == nil {
		return ""
	}
	if s := m.GetConversation(); s != "" {
		return s
	}
	if s := m.GetExtendedTextMessage().GetText(); s != "" {
		return s
	}
	if s := m.GetImageMessage().GetCaption(); s != "" {
		return s
	}
	return m.GetVideoMessage().GetCaption()
}

// digits strips everything but 0-9 (legacy "+62 812…" → "62812…").
func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// validPhone mirrors the authorization rule: 8–15 digits.
func validPhone(s string) bool {
	if len(s) < 8 || len(s) > 15 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

type gatewayError struct{ msg string }

func (e *gatewayError) Error() string { return e.msg }

func errNotConnected() error { return &gatewayError{"Kanal WhatsApp belum terhubung"} }
