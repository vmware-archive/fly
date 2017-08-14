package commands

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/concourse/fly/rc"
)

type CurlCommand struct {
	PositionalArgs struct {
		URL string `positional-arg-name:"url" required:"true" description:"Relative URL"`
	} `positional-args:"yes"`
	Method  string   `short:"X" long:"X" default:"GET" description:"HTTP method"`
	Headers []string `short:"H" long:"H" description:"HTTP headers"`
	Data    string   `short:"d" long:"d" description:"HTTP data"`
}

func (command *CurlCommand) Execute(args []string) error {
	url := command.PositionalArgs.URL
	method := command.Method
	headers := command.Headers
	data := command.Data

	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	client := target.HTTPClient()

	req, err := http.NewRequest(method, target.URL()+url, strings.NewReader(data))
	if err != nil {
		return err
	}

	if err := mergeHeaders(req.Header, headers); err != nil {
		return fmt.Errorf("Error parsing headers: %s", err.Error())
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println(string(body))

	return nil
}

func mergeHeaders(destination http.Header, headerLines []string) error {
	headerString := strings.Join(headerLines, "\n")
	headerString = strings.TrimSpace(headerString)
	headerString += "\n\n"
	headerReader := bufio.NewReader(strings.NewReader(headerString))
	headers, err := textproto.NewReader(headerReader).ReadMIMEHeader()
	if err != nil {
		return err
	}

	for key, values := range headers {
		destination.Del(key)
		for _, value := range values {
			destination.Add(key, value)
		}
	}

	return nil
}
