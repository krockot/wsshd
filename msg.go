package main

import (
	"json"
	"io"
	"fmt"
	"os"
)

const (
	MSG_LOGIN = iota
	MSG_DATA
	MSG_CLOSE
	MSG_WINDOW

	MSG_MAX
)

type MessageType int

type Message struct {
	mtype MessageType
	params map[string]interface{}
}

func (m *Message) Type() MessageType {
	return m.mtype
}

type ParamIntError string
type ParamStringError string

func (e ParamIntError) String() string {
	return fmt.Sprintf("no integer parameter named '%s'", string(e))
}

func (e ParamStringError) String() string {
	return fmt.Sprintf("no string parameter named '%s'", string(e))
}

func (m *Message) ParamInt(name string) (int, os.Error) {
	if vi, ok := m.params[name]; ok {
		if v, ok := vi.(float64); ok {
			return int(v), nil
		}
	}
	return 0, ParamIntError(name)
}

func (m *Message) ParamString(name string) (string, os.Error) {
	if vi, ok := m.params[name]; ok {
		if v, ok := vi.(string); ok {
			return v, nil
		}
	}
	return "", ParamStringError(name)
}

type MessageDecoder struct {
	msg map[string]interface{}
	d *json.Decoder
}

func NewMessageDecoder(r io.Reader) *MessageDecoder {
	return &MessageDecoder{
		msg: make(map[string]interface{}),
		d: json.NewDecoder(r),
	}
}

type InvalidMessageError struct{}

func (e InvalidMessageError) String() string {
	return "invalid message type"
}

func (md *MessageDecoder) DecodeNext() (*Message, os.Error) {
	if err := md.d.Decode(&md.msg); err != nil {
		return nil, err
	}
	if mt, ok := md.msg["t"]; ok {
		m := &Message{params: md.msg}
		mt, ok := mt.(float64)
		if ok && mt < MSG_MAX {
			m.mtype = MessageType(int(mt))
			return m, nil
		} else {
			return nil, &InvalidMessageError{}
		}
	}
	return nil, &InvalidMessageError{}
}

type LoginHandler interface {
	HandleLogin(*string) os.Error
}

type LoginHandlerFunc func(*string) os.Error
func (f LoginHandlerFunc) HandleLogin(un *string) os.Error {
	return f(un)
}

type DataHandler interface {
	HandleData([]byte) os.Error
}

type DataHandlerFunc func([]byte) os.Error
func (f DataHandlerFunc) HandleData(data []byte) os.Error {
	return f(data)
}

type WindowHandler interface {
	HandleWindow(int, int) os.Error
}

type WindowHandlerFunc func(w, h int) os.Error
func (f WindowHandlerFunc) HandleWindow(w, h int) os.Error {
	return f(w, h)
}

type CloseHandler interface {
	HandleClose() os.Error
}

type CloseHandlerFunc func() os.Error
func (f CloseHandlerFunc) HandleClose() os.Error {
	return f()
}

type MessageProcessor struct {
	d *MessageDecoder
	flogin LoginHandler
	fdata DataHandler
	fwindow WindowHandler
	fclose CloseHandler
}

func NewMessageProcessor(r io.Reader) *MessageProcessor {
	return &MessageProcessor{d: NewMessageDecoder(r)}
}

func (p *MessageProcessor) HandleLogin(h LoginHandler) {
	p.flogin = h
}

func (p *MessageProcessor) HandleData(h DataHandler) {
	p.fdata = h
}

func (p *MessageProcessor) HandleClose(h CloseHandler) {
	p.fclose = h
}

func (p *MessageProcessor) HandleWindow(h WindowHandler) {
	p.fwindow = h
}

func (p *MessageProcessor) ProcessNext() os.Error {
	msg, err := p.d.DecodeNext()
	if err != nil {
		return err
	}

	switch(msg.Type()) {
	case MSG_LOGIN:
		if p.flogin != nil {
			username, err := msg.ParamString("u")
			if err != nil {
				return p.flogin.HandleLogin(nil)
			} else {
				return p.flogin.HandleLogin(&username)
			}
		}
	case MSG_DATA:
		if p.fdata != nil {
			b, err := msg.ParamString("b")
			if err != nil {
				return err
			}
			return p.fdata.HandleData([]byte(b))
		}
	case MSG_CLOSE:
		if p.fclose != nil {
			return p.fclose.HandleClose()
		}
	case MSG_WINDOW:
		if p.fwindow != nil {
			w, err := msg.ParamInt("w")
			if err != nil {
				return err
			}
			h, err := msg.ParamInt("h")
			if err != nil {
				return err
			}
			return p.fwindow.HandleWindow(w, h)
		}
	}
	return nil
}

func NewDataMessage(data []byte) *Message {
	m := &Message{mtype: MSG_DATA, params: make(map[string]interface{})}
	m.params["b"] = string(data)
	return m
}

func (m *Message) Encode() ([]byte, os.Error) {
	msg := make(map[string]interface{})
	for k, v := range m.params {
		msg[k] = v
	}
	msg["t"] = m.mtype
	return json.Marshal(msg)
}

