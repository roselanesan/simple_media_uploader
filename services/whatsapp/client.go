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

	_ "github.com/mattn/go-sqlite3"
)

type MessageHandler func(ctx context.Context, sender string, chat string, evt *events.Message) (string, error)

type Client struct {
	client *whatsmeow.Client
	dbPath string
}

func NewClient(dbPath string) *Client {
	return &Client{dbPath: dbPath}
}

func (c *Client) Login(ctx context.Context) error {
	dbLog := waLog.Stdout("database", "ERROR", true)
	container, err := sqlstore.New(ctx, "sqlite3", "file:"+c.dbPath+"?_foreign_keys=on", dbLog)
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
			if v.Info.IsFromMe {
				return
			}

			sender := v.Info.Sender.User
			chat := v.Info.Chat

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
				return
			}

			_, err = c.client.SendMessage(ctx, chat, &proto.Message{
				Conversation: &resp,
			})
			if err != nil {
				slog.Error("send message", "error", err)
			}
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
