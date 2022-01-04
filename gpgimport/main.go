package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	DaemonURL = "ws://localhost:30035"
)

type Credential struct {
	InService    string
	InLogin      string
	OutDirectory string
	OutFilename  string
	Password     string
}

func main() {
	oFlag := flag.String("o", "", "output directory")
	fFlag := flag.String("f", "", "CSV input")
	flag.Parse()

	if len(*oFlag) == 0 || len(*fFlag) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	credentials, err := ParseCSV(*fFlag)
	if err != nil {
		log.Fatal(err)
	}
	
	dialer := &websocket.Dialer{
		HandshakeTimeout: 2 * time.Second,
	}

	wsconn, _, err := dialer.Dial(DaemonURL, nil)
	if err != nil {
		log.Fatalf("cannot connect to the moolticute daemon: %w", err)
	}
	defer wsconn.Close()

	if err = os.MkdirAll(*oFlag, 0700); err != nil {
		log.Fatalf("cannot create output directory: %w", err)
	}

	for _, c := range credentials {
		c.Password, err = FetchPassword(wsconn, c.InService, c.InLogin)
		if err != nil {
			log.Fatalf("Cannot fetch password for %s/%s: %w", c.InService, c.InLogin, err)
		}
	}
	
	for _, c := range credentials {
		fmt.Printf("%s/%s: %d characters\n", c.InService, c.InLogin, len(c.Password))
	}
}

func ParseCSV(filename string) ([]*Credential, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var credentials []*Credential

	scanner := bufio.NewScanner(f)
	linenr := 0

	for scanner.Scan() {
		linenr++
		fields := strings.Split(scanner.Text(), ";")
		if len(fields) != 4 {
			return nil, fmt.Errorf("%s:%d: expected 4 fields, got %d", filename, linenr, len(fields))
		}

		c := &Credential{
			InService:    Unquote(fields[0]),
			InLogin:      Unquote(fields[1]),
			OutDirectory: Unquote(fields[2]),
			OutFilename:  Unquote(fields[3]),
		}

		if len(c.OutDirectory) == 0 {
			c.OutDirectory = c.InService
		}

		if len(c.OutFilename) == 0 {
			c.OutFilename = c.InLogin
		}

		credentials = append(credentials, c)
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return credentials, nil
}

func Unquote(s string) string {
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}

	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}

	return s
}

type Header struct {
	MessageType string `json:"msg"`
}

type CredentialData struct {
	Header
	Data struct {
		Service  string `json:"service"`
		Login    string `json:"login"`
		Password string `json:"password"`
		Failed   bool   `json:"failed"`
	} `json:"data"`
}

func FetchPassword(wsconn *websocket.Conn, service string, login string) (string, error) {
	msg := fmt.Sprintf(`{"msg":"get_credential","data":{"service":"%s","login":"%s"}}`, service, login)
	if err := wsconn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return "", fmt.Errorf("WriteMessage: %w", err)
	}

	var (
		header Header
		cdata  CredentialData
	)

	for {
		mtype, data, err := wsconn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("ReadMessage: %w\n", err)
		}

		if mtype != websocket.TextMessage {
			continue
		}

		if err = json.Unmarshal(data, &header); err != nil {
			return "", fmt.Errorf("Unmarshal header: %w\n", err)
		}

		if header.MessageType != "get_credential" {
			continue
		}

		if err = json.Unmarshal(data, &cdata); err != nil {
			return "", fmt.Errorf("Unmarshal credential data: %w\n", err)
		}

		if cdata.Data.Failed {
			return "", fmt.Errorf("Operation canceled by user")
		}

		if cdata.Data.Service != service || cdata.Data.Login != login {
			return "", fmt.Errorf("Received password for wrong service/login")
		}

		return cdata.Data.Password, nil
	}
}
