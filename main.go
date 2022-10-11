package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"

	"go.lsp.dev/protocol"
)

const LogFile = "/tmp/lspserver.log"

type Logger struct {
	writer io.WriteCloser
}

func (l Logger) Close() {
	l.writer.Close()
}

func (l Logger) Infof(format string, params ...any) {
	fmt.Fprintf(l.writer, fmt.Sprintf("%s\n", format), params...)
}

func (l Logger) Info(params ...any) {
	fmt.Fprintln(l.writer, params...)
}

func newLogger() (Logger, error) {
	f, err := os.OpenFile(LogFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open log file: %s", err)
		return Logger{}, err
	}

	return Logger{writer: f}, nil
}

type Reader struct {
	in bufio.Reader
}

var ContentLengthRegex = regexp.MustCompile(`\A[Cc]ontent-[Ll]ength: (?P<length>\d+)`)

func findNamedMatches(regex *regexp.Regexp, str string) map[string]string {
	match := regex.FindStringSubmatch(str)

	results := map[string]string{}
	for i, name := range match {
		results[regex.SubexpNames()[i]] = name
	}
	return results
}

func (r Reader) Read() ([]byte, error) {
	input := ""
	for {
		line, err := r.in.ReadString('\n')
		if err != nil {
			return nil, err
		}
		log.Info(line)
		input += line
		if len(input) >= 4 && input[len(input)-4:] == "\r\n\r\n" {
			break
		}
	}

	matches := findNamedMatches(ContentLengthRegex, input)
	lengthStr, found := matches["length"]
	if !found {
		return nil, fmt.Errorf("cannot parse content length for. %s", input)
	}

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return nil, err
	}

	payload := make([]byte, length)

	n, err := r.in.Read(payload)
	if err != nil {
		return nil, err
	}
	log.Info(string(payload))
	if n != length {
		return nil, fmt.Errorf("payload should be of length %d, but is of length %d", length, n)
	}

	return payload, nil
}

var log Logger

func main() {
	var err error

	log, err = newLogger()
	if err != nil {
		os.Exit(1)
	}
	defer log.Close()

	log.Info("Started.")

	run()
}

func run() {
	reader := Reader{in: *bufio.NewReader(os.Stdin)}
	server := Server{}
	for {
		request, err := reader.Read()
		if err != nil {
			log.Info("Error: ", err)
		}
		log.Info("IN: ", string(request))

		server.Handle(request)
	}
}

type Server struct {
}

type Request struct {
	Method string          `json:"method"`
	ID     uint64          `json:"id"`
	Params json.RawMessage `json:"params"`
}

func (s *Server) Handle(msg []byte) {
	var r Request
	err := json.Unmarshal(msg, &r)
	if err != nil {
		log.Info("While unmarshalling request: ", err)
		return
	}

	switch r.Method {
	case protocol.MethodInitialize:
		var params protocol.InitializeParams
		err := json.Unmarshal(r.Params, &params)
		if err != nil {
			log.Info("While unmarshalling initialize: ", err)
			return
		}
		s.onInitialize(&params)
	}
}

func (s *Server) onInitialize(params *protocol.InitializeParams) {
	log.Infof("%#v", params)
}
