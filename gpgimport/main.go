package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	DaemonURL = "ws://localhost:30035"
)

type Credential struct {
	InService    string
	InLogin      string
	OutDirectory string
	OutFilename  string
}

type Header struct {
	MessageType string `json:"msg"`
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

	for _, c := range credentials {
		fmt.Printf("Service \"%s\", Login \"%s\" stored under \"%s\" as \"%s\"\n",
		c.InService, c.InLogin, c.OutDirectory, c.OutFilename)
	}
}

func ParseCSV(filename string) ([]Credential, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var credentials []Credential

	scanner := bufio.NewScanner(f)
	linenr := 0

	for scanner.Scan() {
		linenr++
		fields := strings.Split(scanner.Text(), ";")
		if len(fields) != 4 {
			return nil, fmt.Errorf("%s:%d: expected 4 fields, got %d", filename, linenr, len(fields))
		}

		c := Credential{
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
