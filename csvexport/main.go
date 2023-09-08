package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	DaemonURL = "ws://localhost:30035"
)

type Header struct {
	MessageType string `json:"msg"`
}

type ModeChange struct {
	Header
	Unlocked bool `json:"data"`
}

type MemData struct {
	Header
	Data struct {
		LoginNodes []struct {
			Service string `json:"service"`
			Logins []struct {
				Login string `json:"login"`
			} `json:"childs"`
		} `json:"login_nodes"`
	} `json:"data"`
}

func main() {
	dialer := &websocket.Dialer{
		HandshakeTimeout: 2 * time.Second,
	}

	wsconn, _, err := dialer.Dial(DaemonURL, nil)
	if err != nil {
		log.Fatalf("cannot connect to the moolticute daemon: %v\n", err)
	}
	defer wsconn.Close()

	msg := []byte(`{"msg":"start_memorymgmt"}`)
	if err = wsconn.WriteMessage(websocket.TextMessage, msg); err != nil {
		log.Fatalf("WriteMessage: %v\n", err)
	}

	var (
		header Header
		memData MemData
		modeChange ModeChange
		unlocked bool
	)

	for {
		mtype, data, err := wsconn.ReadMessage()
		if err != nil {
			log.Fatalf("ReadMessage: %v\n", err)
		}

		if mtype != websocket.TextMessage {
			log.Printf("unknown message type %d\n", mtype)
			continue
		}

		if err = json.Unmarshal(data, &header); err != nil {
			log.Fatalf("Unmarshal header: %v\n", err)
		}

		if header.MessageType == "failed_memorymgmt" {
			log.Fatalf("Operation canceled by user")
		}

		if !unlocked {
			if header.MessageType != "memorymgmt_changed" {
				continue
			}

			if err = json.Unmarshal(data, &modeChange); err != nil {
				log.Fatalf("Unmarshal mode change: %v\n", err)
			}

			unlocked = modeChange.Unlocked
			continue
		}

		if header.MessageType != "memorymgmt_data" {
			continue
		}

		if err = json.Unmarshal(data, &memData); err != nil {
			log.Fatalf("Unmarshal memory data: %v\n", err)
		}

		for _, service := range memData.Data.LoginNodes {
			for _, login := range service.Logins {
				fmt.Printf("\"%s\";\"%s\"\n", service.Service, login.Login)
			}
		}
		
		break
	}
}
