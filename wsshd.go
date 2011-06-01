package main


import(
	"github.com/krockot/goterm/term"
	"websocket"
	"http"
	"io"
	"os"
	"syscall"
	"time"
	"flag"
	"fmt"
)

type Client struct {
	conn io.ReadWriteCloser
	tty *term.Terminal
	pid int
	quit chan bool
}

func (c *Client) Close() os.Error {
	if c.tty != nil {
		c.tty.Close()
	}
	if c.pid > 0 {
		syscall.Kill(c.pid, 1)
		os.Wait(c.pid, 0)
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
			c.quit <- true
		}()

		go func() {
			t := time.NewTicker(1e9)
			for {
				select {
				case <-c.quit:
					c.Close()
					return
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
	c := &Client{conn: ws, quit: make(chan bool, 2)}

	mp := NewMessageProcessor(ws)
	mp.HandleLogin(HandleLogin(c))
	mp.HandleData(HandleData(c))
	mp.HandleClose(HandleClose(c))
	mp.HandleWindow(HandleWindow(c))
	for {
		if err := mp.ProcessNext(); err != nil {
			c.quit <- true
			return
		}
	}
}

func main() {
	certFile := flag.String("cert", "", "Path to certificate file")
	keyFile := flag.String("key", "", "Path to private key file")
	port := flag.String("port", "8022", "Service port")
	dontCare := flag.Bool("dontcare", false, "Force non-secure mode if no cert or key is given")
	flag.Parse()

	notSoGood := false
	if certFile == nil || keyFile == nil {
		if *dontCare {
			notSoGood = true
		} else {
			fmt.Fprintln(os.Stderr,
				"Cannot use TLS without a cert and key.  Use -dontcare to serve without encrpytion.")
			return
		}
	}

	http.Handle("/sh", websocket.Handler(ShellHandler))

	var err os.Error
	if notSoGood {
		err = http.ListenAndServe(":" + *port, nil)
	} else {
		err = http.ListenAndServeTLS(":" + *port, *certFile, *keyFile, nil)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.String())
	}
}

