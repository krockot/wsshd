package main


import(
	"github.com/krockot/goterm/term"
	"websocket"
	"http"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

type Client struct {
	conn io.ReadWriteCloser
	tty *term.Terminal
	pid int
}

func (c *Client) Close() os.Error {
	if c.pid > 0 {
		syscall.Kill(c.pid, 1)
		os.Wait(c.pid, os.WNOHANG)
	}
	if c.tty != nil {
		c.tty.Close()
	}
	c.conn.Close()
	return nil
}

func HandleLogin(c *Client) LoginHandlerFunc {
	return func(username *string) os.Error {
		tty, pid, err := term.ForkPty(
			"/bin/login",
			[]string{"/bin/login"},
			term.DefaultAttributes(),
			term.NewWindowSize(80,25))
		if err != nil {
			return err
		}

		c.tty = tty
		c.pid = pid

		go func() {
			buffer := make([]byte, 1024)
			for {
				n, err := tty.Read(buffer)
				if err != nil {
					break
				}
				msg := NewDataMessage(buffer[:n])
				data, err := msg.Encode()
				if err != nil {
					break
				}
				n, err = c.conn.Write(data)
				if err != nil {
					break
				}
			}
			c.Close()
		}()

		go func() {
			t := time.NewTicker(1e9)
			for {
				select {
				case <-t.C:
					msg, err := os.Wait(c.pid, os.WNOHANG)
					if err == nil && msg.Pid == c.pid {
						c.Close()
						return
					}
				}
			}
		}()
		return nil
	}
}

func HandleData(c *Client) DataHandlerFunc {
	return func(data []byte) os.Error {
		if c.tty != nil {
			_, err := c.tty.Write(data)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func HandleClose(c *Client) CloseHandlerFunc {
	return func() os.Error {
		return c.Close()
	}
}

func HandleWindow(c *Client) WindowHandlerFunc {
	return func(w, h int) os.Error {
		if c.tty != nil {
			return c.tty.SetWindowSize(term.NewWindowSize(uint16(w), uint16(h)))
		}
		return nil
	}
}

func ShellHandler(ws *websocket.Conn) {
	c := &Client{conn: ws}

	mp := NewMessageProcessor(ws)
	mp.HandleLogin(HandleLogin(c))
	mp.HandleData(HandleData(c))
	mp.HandleClose(HandleClose(c))
	mp.HandleWindow(HandleWindow(c))
	for {
		if err := mp.ProcessNext(); err != nil {
			fmt.Printf("Closing: %s\n", err.String())
			return
		}
	}
}

func main() {
	http.Handle("/sh", websocket.Handler(ShellHandler))
	http.ListenAndServe(":8022", nil)
}

