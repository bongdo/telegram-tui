package tgclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/query/messages"
	"github.com/gotd/td/tg"
)

// DownloadMedia fetches a voice note, round note or photo into the cache and
// emits MediaReadyEvent. The caller decides what to do with the path.
func (b *Bridge) DownloadMedia(key string, msgID int) {
	b.mu.RLock()
	api := b.api
	peer := b.peers[key]
	b.mu.RUnlock()
	if api == nil || peer == nil {
		b.emit(MediaReadyEvent{PeerKey: key, MessageID: msgID, Err: errors.New("chat is not ready")})
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(b.context(), 3*time.Minute)
		defer cancel()

		raw, err := b.fetchMessage(ctx, peer, msgID)
		if err != nil {
			b.emit(MediaReadyEvent{PeerKey: key, MessageID: msgID, Err: err})
			return
		}
		file, ok := messages.Elem{Msg: raw}.File()
		if !ok {
			b.emit(MediaReadyEvent{PeerKey: key, MessageID: msgID, Err: errors.New("message has no downloadable media")})
			return
		}

		kind := mediaKind(raw)
		name := file.Name
		if name == "" {
			name = "media"
		}
		name = sanitizeFilename(name)
		path := filepath.Join(b.cfg.CacheDir, fmt.Sprintf("%s_%d_%s", sanitizeFilename(key), msgID, name))

		if _, err := os.Stat(path); err != nil {
			if _, err := downloader.NewDownloader().Download(api, file.Location).ToPath(ctx, path); err != nil {
				b.emit(MediaReadyEvent{PeerKey: key, MessageID: msgID, Kind: kind, Err: err})
				return
			}
		}
		if b.store != nil {
			_ = b.store.SetMedia(key, msgID, kind, path)
		}
		b.emit(MediaReadyEvent{PeerKey: key, MessageID: msgID, Kind: kind, Path: path})
	}()
}

func (b *Bridge) fetchMessage(ctx context.Context, input tg.InputPeerClass, id int) (*tg.Message, error) {
	var (
		res tg.MessagesMessagesClass
		err error
	)
	if channel, ok := input.(*tg.InputPeerChannel); ok {
		res, err = b.api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: channel.ChannelID, AccessHash: channel.AccessHash},
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: id}},
		})
	} else {
		res, err = b.api.MessagesGetMessages(ctx, []tg.InputMessageClass{&tg.InputMessageID{ID: id}})
	}
	if err != nil {
		return nil, err
	}
	var raw []tg.MessageClass
	switch m := res.(type) {
	case *tg.MessagesMessages:
		raw = m.Messages
	case *tg.MessagesMessagesSlice:
		raw = m.Messages
	case *tg.MessagesChannelMessages:
		raw = m.Messages
	}
	for _, item := range raw {
		if msg, ok := item.(*tg.Message); ok && msg.ID == id {
			return msg, nil
		}
	}
	return nil, errors.New("message not found")
}

func mediaKind(msg *tg.Message) string {
	switch m := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		if voiceOf(m) != nil {
			return "voice"
		}
		if roundOf(m) != nil {
			return "round"
		}
		return "file"
	default:
		return "file"
	}
}

func sanitizeFilename(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, s)
	if s == "" {
		return strconv.Itoa(int(time.Now().UnixNano()))
	}
	return s
}
