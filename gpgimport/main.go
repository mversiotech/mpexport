package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	DaemonURL = "ws://localhost:30035"
	GpgBinary = "/usr/bin/gpg2"
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
	rFlag := flag.String("r", "", "GPG recipient")
	flag.Parse()

	if len(*oFlag) == 0 || len(*fFlag) == 0 || len(*rFlag) == 0 {
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
		log.Fatalf("cannot connect to the moolticute daemon: %v", err)
	}
	defer wsconn.Close()

	for _, c := range credentials {
		c.Password, err = FetchPassword(wsconn, c.InService, c.InLogin)
		if err != nil {
			log.Fatalf("Cannot fetch password for %s/%s: %v", c.InService, c.InLogin, err)
		}
	}

	for _, c := range credentials {
		if err = SaveEncrypted(c, *oFlag, *rFlag); err != nil {
			log.Fatalf("Cannot store GPG-encrypted data for %s/%s: %v", c.InService, c.InLogin, err)
		}
	}

	fmt.Println("Done")
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
		if len(fields) < 2 || len(fields) > 4 {
			return nil, fmt.Errorf("%s:%d: expected 4 fields, got %d", filename, linenr, len(fields))
		}

		c := &Credential{
			InService: Unquote(fields[0]),
			InLogin:   Unquote(fields[1]),
		}

		if len(fields) > 3 {
			c.OutDirectory = Unquote(fields[2])
		} else {
			c.OutDirectory = c.InService
		}

		if len(fields) == 4 {
			c.OutFilename = Unquote(fields[3])
		} else {
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
		return "", fmt.Errorf("WriteMessage: %v", err)
	}

	var (
		header Header
		cdata  CredentialData
	)

	for {
		mtype, data, err := wsconn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("ReadMessage: %v\n", err)
		}

		if mtype != websocket.TextMessage {
			continue
		}

		if err = json.Unmarshal(data, &header); err != nil {
			return "", fmt.Errorf("Unmarshal header: %v\n", err)
		}

		if header.MessageType != "get_credential" {
			continue
		}

		if err = json.Unmarshal(data, &cdata); err != nil {
			return "", fmt.Errorf("Unmarshal credential data: %v\n", err)
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

func SaveEncrypted(c *Credential, basedir string, recipient string) error {
	var content bytes.Buffer
	fmt.Fprintf(&content, "%s\nlogin: %s\n", c.Password, c.InLogin)

	outdir := filepath.Join(basedir, c.OutDirectory)

	if err := os.MkdirAll(outdir, 0700); err != nil {
		return fmt.Errorf("cannot create output directory: %v", err)
	}

	gpgfile := filepath.Join(outdir, c.OutFilename+".gpg")

	var stderr bytes.Buffer

	cmd := exec.Command(
		GpgBinary,
		"--batch",
		"--use-agent",
		"-o",
		gpgfile,
		"-e",
		"-r",
		recipient)

	cmd.Stderr = &stderr
	cmd.Stdin = &content

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v\nStderr:\n%s", err, stderr.String())
	}

	return nil
}
