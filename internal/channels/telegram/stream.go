package telegram

import (
	"context"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	// defaultStreamThrottle is the minimum delay between message edits (matching TS: 1000ms).
	defaultStreamThrottle = 1000 * time.Millisecond

	// streamMaxChars is the max message length for streaming (Telegram limit).
	streamMaxChars = 4096

	// draftIDMax is the maximum value for draft_id before wrapping.
	draftIDMax = math.MaxInt32
)

// nextDraftID is a global atomic counter for sendMessageDraft draft_id values.
// Each streaming session gets a unique ID (matching TS pattern: 1 → Int32 max, wraps).
var nextDraftID atomic.Int32

// allocateDraftID returns a unique draft_id for sendMessageDraft.
func allocateDraftID() int {
	for {
		cur := nextDraftID.Load()
		next := cur + 1
		if next >= int32(draftIDMax) {
			next = 1
		}
		if nextDraftID.CompareAndSwap(cur, next) {
			return int(next)
		}
	}
}

// draftFallbackRe matches Telegram API errors indicating sendMessageDraft is unsupported.
// Ref: TS src/telegram/draft-stream.ts fallback patterns.
var draftFallbackRe = regexp.MustCompile(`(?i)(unknown method|method.*not (found|available|supported)|unsupported|can't be used|can be used only)`)

// shouldFallbackFromDraft returns true if the error indicates sendMessageDraft
// is permanently unavailable and the stream should fall back to message transport.
func shouldFallbackFromDraft(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "sendmessagedraft") && !strings.Contains(msg, "send_message_draft") {
		return false
	}
	return draftFallbackRe.MatchString(err.Error())
}

// DraftStream manages a streaming preview message that gets edited as content arrives.
// Ref: TS src/telegram/draft-stream.ts → createTelegramDraftStream()
//
// Supports two transports:
//   - Draft transport (sendMessageDraft): Preferred for DMs. Ephemeral preview, no real message created.
//   - Message transport (sendMessage + editMessageText): Fallback. Creates a real message that can be edited.
//
// State machine:
//
//	NOT_STARTED → first Update() → sendMessageDraft or sendMessage → STREAMING
//	STREAMING   → subsequent Update() → sendMessageDraft or editMessageText (throttled) → STREAMING
//	STREAMING   → Stop() → final flush → STOPPED
//	STREAMING   → Clear() → deleteMessage (message transport only) → DELETED
type DraftStream struct {
	bot             *telego.Bot
	chatID          int64
	messageThreadID int           // forum topic thread ID (0 = no thread)
	messageID       int           // 0 = not yet created (message transport only)
	lastText        string        // last sent text (for dedup)
	throttle        time.Duration // min delay between edits
	lastEdit        time.Time
	mu              sync.Mutex
	stopped         bool
	pending         string // pending text to send (buffered during throttle)
	draftID         int    // sendMessageDraft draft_id (0 = message transport)
	useDraft        bool   // true = draft transport, false = message transport
	draftFailed     bool   // true = draft API rejected permanently, using message transport
}

// newDraftStream creates a new streaming preview manager.
// When useDraft is true, the stream will attempt to use sendMessageDraft (Bot API 9.3+)
// and automatically fall back to sendMessage+editMessageText if the API rejects it.
func newDraftStream(bot *telego.Bot, chatID int64, throttleMs int, messageThreadID int, useDraft bool) *DraftStream {
	throttle := defaultStreamThrottle
	if throttleMs > 0 {
		throttle = time.Duration(throttleMs) * time.Millisecond
	}
	var draftID int
	if useDraft {
		draftID = allocateDraftID()
	}
	return &DraftStream{
		bot:             bot,
		chatID:          chatID,
		messageThreadID: messageThreadID,
		throttle:        throttle,
		useDraft:        useDraft,
		draftID:         draftID,
	}
}

// Update sends or edits the streaming message with the latest text.
// Throttled to avoid hitting Telegram rate limits.
func (ds *DraftStream) Update(ctx context.Context, text string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.stopped {
		return
	}

	// Truncate to Telegram max
	if len(text) > streamMaxChars {
		text = text[:streamMaxChars]
	}

	// Dedup: skip if text unchanged
	if text == ds.lastText {
		return
	}

	ds.pending = text

	// Check throttle
	if time.Since(ds.lastEdit) < ds.throttle {
		return
	}

	ds.flush(ctx)
}

// Flush forces sending the pending text immediately.
func (ds *DraftStream) Flush(ctx context.Context) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.flush(ctx)
}

// flush sends/edits the pending text (must hold mu lock).
func (ds *DraftStream) flush(ctx context.Context) error {
	if ds.pending == "" || ds.pending == ds.lastText {
		return nil
	}

	text := ds.pending
	htmlText := markdownToTelegramHTML(text)

	// --- Draft transport (sendMessageDraft) ---
	if ds.useDraft && !ds.draftFailed {
		params := &telego.SendMessageDraftParams{
			ChatID:    ds.chatID,
			DraftID:   ds.draftID,
			Text:      htmlText,
			ParseMode: telego.ModeHTML,
		}
		if sendThreadID := resolveThreadIDForSend(ds.messageThreadID); sendThreadID > 0 {
			params.MessageThreadID = sendThreadID
		}
		if err := ds.bot.SendMessageDraft(ctx, params); err != nil {
			if shouldFallbackFromDraft(err) {
				// Permanent fallback to message transport
				slog.Warn("stream: sendMessageDraft unavailable, falling back to message transport", "error", err)
				ds.draftFailed = true
				// Fall through to message transport below
			} else {
				slog.Debug("stream: sendMessageDraft failed", "error", err)
				return err
			}
		} else {
			ds.lastText = text
			ds.lastEdit = time.Now()
			return nil
		}
	}

	// --- Message transport (sendMessage + editMessageText) ---
	if ds.messageID == 0 {
		// First message: send new
		// TS ref: buildTelegramThreadParams() — General topic (1) must be omitted.
		params := &telego.SendMessageParams{
			ChatID:    tu.ID(ds.chatID),
			Text:      htmlText,
			ParseMode: telego.ModeHTML,
		}
		if sendThreadID := resolveThreadIDForSend(ds.messageThreadID); sendThreadID > 0 {
			params.MessageThreadID = sendThreadID
		}
		msg, err := ds.bot.SendMessage(ctx, params)
		// TS ref: withTelegramThreadFallback — retry without thread ID when topic is deleted.
		if err != nil && params.MessageThreadID != 0 && threadNotFoundRe.MatchString(err.Error()) {
			slog.Warn("stream: thread not found, retrying without message_thread_id", "thread_id", params.MessageThreadID)
			params.MessageThreadID = 0
			msg, err = ds.bot.SendMessage(ctx, params)
		}
		if err != nil {
			slog.Debug("stream: failed to send initial message", "error", err)
			return err
		}
		ds.messageID = msg.MessageID
	} else {
		// Edit existing message
		editMsg := tu.EditMessageText(tu.ID(ds.chatID), ds.messageID, htmlText)
		editMsg.ParseMode = telego.ModeHTML
		if _, err := ds.bot.EditMessageText(ctx, editMsg); err != nil {
			// Ignore "not modified" errors
			if !messageNotModifiedRe.MatchString(err.Error()) {
				slog.Debug("stream: failed to edit message", "error", err)
			}
		}
	}

	ds.lastText = text
	ds.lastEdit = time.Now()
	return nil
}

// Stop finalizes the stream with a final edit.
func (ds *DraftStream) Stop(ctx context.Context) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.stopped = true
	return ds.flush(ctx)
}

// Clear stops the stream and deletes the message (message transport only).
// Draft transport has no persistent message to delete.
func (ds *DraftStream) Clear(ctx context.Context) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.stopped = true
	if ds.messageID != 0 {
		_ = ds.bot.DeleteMessage(ctx, &telego.DeleteMessageParams{
			ChatID:    tu.ID(ds.chatID),
			MessageID: ds.messageID,
		})
		ds.messageID = 0
	}
	return nil
}

// MessageID returns the streaming message ID (0 if not yet created or using draft transport).
func (ds *DraftStream) MessageID() int {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.messageID
}

// UsedDraftTransport returns true if the stream is (or was) using draft transport
// and didn't fall back to message transport.
func (ds *DraftStream) UsedDraftTransport() bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.useDraft && !ds.draftFailed
}

// --- StreamingChannel implementation ---

// OnStreamStart prepares for streaming.
// chatID here is the localKey (composite key with :topic:N suffix for forum topics).
//
// For DMs: seeds the stream with the "Thinking..." placeholder messageID so that
// flush() uses editMessageText to update it progressively. This gives a smooth
// transition: "Thinking..." → streaming chunks → (Send() edits final formatted response).
//
// For groups: deletes the placeholder and lets the stream create its own message,
// since group placeholders drift away as other messages arrive.
func (c *Channel) OnStreamStart(ctx context.Context, chatID string) error {
	id, err := parseRawChatID(chatID)
	if err != nil {
		return err
	}

	// Look up thread ID stored during handleMessage
	threadID := 0
	if v, ok := c.threadIDs.Load(chatID); ok {
		threadID = v.(int)
	}

	isDM := id > 0

	// Both DMs and groups use message transport (editMessageText).
	// sendMessageDraft (draft transport) is available in the codebase but disabled
	// because it causes "reply to deleted message" artifacts in Telegram clients.
	ds := newDraftStream(c.bot, id, 0, threadID, false)

	if isDM {
		// DMs: seed the stream with the "Thinking..." placeholder messageID.
		// flush() will use editMessageText to update it progressively.
		if pID, ok := c.placeholders.Load(chatID); ok {
			c.placeholders.Delete(chatID)
			ds.messageID = pID.(int)
			slog.Info("stream: DM using placeholder for progressive edit", "chat_id", id, "message_id", pID.(int))
		} else {
			slog.Info("stream: DM starting stream (no placeholder found)", "chat_id", id)
		}
	} else {
		// Groups: delete placeholder — the stream creates its own message.
		if pID, ok := c.placeholders.Load(chatID); ok {
			c.placeholders.Delete(chatID)
			_ = c.deleteMessage(ctx, id, pID.(int))
		}
		slog.Info("stream: group using message transport", "chat_id", id)
	}

	c.streams.Store(chatID, ds)

	return nil
}

// OnChunkEvent updates the streaming message with accumulated content.
func (c *Channel) OnChunkEvent(ctx context.Context, chatID string, fullText string) error {

	val, ok := c.streams.Load(chatID)
	if !ok {
		return nil
	}

	ds := val.(*DraftStream)
	ds.Update(ctx, fullText)
	return nil
}

// OnStreamEnd finalizes the streaming preview.
// Hands the stream's messageID back to the placeholders map so that Send()
// can edit it with the properly formatted final response.
func (c *Channel) OnStreamEnd(ctx context.Context, chatID string, _ string) error {
	val, ok := c.streams.Load(chatID)
	if !ok {
		return nil
	}

	ds := val.(*DraftStream)

	// Mark stream as stopped (no more edits)
	ds.mu.Lock()
	ds.stopped = true
	msgID := ds.messageID
	ds.mu.Unlock()

	c.streams.Delete(chatID)

	if msgID != 0 {
		// Hand off the stream message to Send() for final formatted edit.
		c.placeholders.Store(chatID, msgID)
		slog.Info("stream: ended, handing off to Send()", "chat_id", chatID, "message_id", msgID)
	}

	// Stop thinking animation
	if stop, ok := c.stopThinking.Load(chatID); ok {
		if cf, ok := stop.(*thinkingCancel); ok {
			cf.Cancel()
		}
		c.stopThinking.Delete(chatID)
	}

	return nil
}
