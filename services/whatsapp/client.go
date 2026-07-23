package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "modernc.org/sqlite"
)

type MessageHandler func(ctx context.Context, sender string, chat string, evt *events.Message) (string, error)

type Client struct {
	client       *whatsmeow.Client
	dbPath       string
	disconnected chan struct{}
}

func NewClient(dbPath string) *Client {
	return &Client{dbPath: dbPath, disconnected: make(chan struct{}, 1)}
}

func (c *Client) Login(ctx context.Context) error {
	dbLog := waLog.Stdout("database", "ERROR", true)
	container, err := sqlstore.New(ctx, "sqlite", "file:"+c.dbPath+"?_pragma=foreign_keys(1)", dbLog)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("get device: %w", err)
	}

	c.client = whatsmeow.NewClient(deviceStore, waLog.Stdout("client", "ERROR", true))

	if c.client.Store.ID == nil {
		return c.loginWithQR(ctx)
	}
	return c.connect(ctx)
}

func (c *Client) loginWithQR(ctx context.Context) error {
	qrChan, err := c.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("qr channel: %w", err)
	}
	if err := c.client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	for evt := range qrChan {
		switch evt.Event {
		case "code":
			fmt.Println("Scan this QR code with WhatsApp:")
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
		case "timeout":
			return fmt.Errorf("QR scan timed out")
		default:
			fmt.Printf("Login status: %s\n", evt.Event)
		}
	}
	return nil
}

func (c *Client) connect(ctx context.Context) error {
	if err := c.client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	fmt.Printf("Logged in as %s\n", c.client.Store.ID.User)
	return nil
}

func (c *Client) Start(ctx context.Context, handler MessageHandler) error {
	c.client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			slog.Debug("event received",
				"is_from_me", v.Info.IsFromMe,
				"sender", v.Info.Sender.User,
				"chat", v.Info.Chat.String(),
				"has_image", v.Message.GetImageMessage() != nil,
				"has_video", v.Message.GetVideoMessage() != nil,
				"has_doc", v.Message.GetDocumentMessage() != nil,
			)

			if v.Info.IsFromMe {
				return
			}

			sender := v.Info.Sender.User
			chat := v.Info.Chat

			slog.Info("incoming message",
				"from", sender,
				"chat", chat.String(),
				"has_media", v.Message.GetImageMessage() != nil || v.Message.GetVideoMessage() != nil || v.Message.GetDocumentMessage() != nil,
			)

			handlerCtx := ctx
			if err := ctx.Err(); err != nil {
				handlerCtx = context.Background()
			}

			resp, err := handler(handlerCtx, sender, chat.String(), v)
			if err != nil {
				slog.Error("handler error", "error", err)
				return
			}
			if resp == "" {
				slog.Info("handler returned empty response (likely not whitelisted or unsupported message)", "from", sender)
				return
			}

			slog.Info("sending reply", "to", sender, "response_length", len(resp))
			_, err = c.client.SendMessage(ctx, chat, &proto.Message{
				Conversation: &resp,
			})
			if err != nil {
				slog.Error("send message", "error", err)
			}
		case *events.Disconnected:
			slog.Warn("WhatsApp disconnected")
			select {
			case c.disconnected <- struct{}{}:
			default:
			}
		case *events.Connected:
			slog.Info("WhatsApp connected")
		}
	})
	return nil
}

func (c *Client) Logout() {
	if c.client != nil {
		c.client.Disconnect()
	}
}

func (c *Client) IsLoggedIn() bool {
	return c.client != nil && c.client.Store.ID != nil
}

func (c *Client) Download(ctx context.Context, msg whatsmeow.DownloadableMessage) ([]byte, error) {
	return c.client.Download(ctx, msg)
}

func (c *Client) Disconnected() <-chan struct{} {
	return c.disconnected
}
