package tgclient

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"
)

func TestApplyUpdatesFromSendResponse(t *testing.T) {
	b := New(Config{})
	b.ctx = context.Background()
	d := tg.NewUpdateDispatcher()
	b.registerHandlers(d)
	b.dispatcher = &d

	b.applyUpdates(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{
			&tg.UpdateNewMessage{Message: &tg.Message{
				ID:      41,
				PeerID:  &tg.PeerUser{UserID: 7},
				Message: "hello",
				Out:     true,
				Date:    1700000000,
			}},
		},
		Users: []tg.UserClass{&tg.User{ID: 7, FirstName: "Me", Self: true}},
	})

	select {
	case ev := <-b.events:
		msg, ok := ev.(NewMessageEvent)
		if !ok {
			t.Fatalf("event = %T", ev)
		}
		if msg.PeerKey != "user:7" || msg.Message.ID != 41 || !msg.Message.Out || msg.Message.Text != "hello" {
			t.Fatalf("event = %+v", msg)
		}
	default:
		t.Fatal("no event for own sent message")
	}
}

func TestApplyUpdatesReaction(t *testing.T) {
	b := New(Config{})
	b.ctx = context.Background()
	d := tg.NewUpdateDispatcher()
	b.registerHandlers(d)
	b.dispatcher = &d

	b.applyUpdates(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{
			&tg.UpdateMessageReactions{
				Peer:  &tg.PeerChannel{ChannelID: 99},
				MsgID: 12,
				Reactions: tg.MessageReactions{
					Results: []tg.ReactionCount{
						{Reaction: &tg.ReactionEmoji{Emoticon: "👍"}, Count: 2},
						{Flags: 1, ChosenOrder: 0, Reaction: &tg.ReactionEmoji{Emoticon: "🔥"}, Count: 1},
					},
				},
			},
		},
	})

	ev := <-b.events
	re, ok := ev.(ReactionsEvent)
	if !ok {
		t.Fatalf("event = %T", ev)
	}
	if re.PeerKey != "channel:99" || re.MessageID != 12 || len(re.Reactions) != 2 {
		t.Fatalf("event = %+v", re)
	}
	if re.Reactions[1].Emoji != "🔥" || !re.Reactions[1].Mine {
		t.Fatalf("chosen reaction = %+v", re.Reactions[1])
	}
	if re.Reactions[0].Mine {
		t.Fatalf("unselected reaction marked mine: %+v", re.Reactions[0])
	}
}
