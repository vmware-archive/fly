package commands

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type LoginCommand struct {
	ATCURL   string `short:"c" long:"concourse-url" description:"Concourse URL to authenticate with"`
	Insecure bool   `short:"k" long:"insecure"      description:"Skip verification of the endpoint's SSL certificate"`
	Username string `short:"u" long:"username"      description:"Username for basic auth"`
	Password string `short:"p" long:"password"      description:"Password for basic auth"`
	Token    string `          long:"token"         description:"Token for OAuth"`
}

func (command *LoginCommand) Execute(args []string) error {
	if Fly.Target == "" {
		return errors.New("name for the target must be specified (--target/-t)")
	}

	var client concourse.Client
	var err error

	if command.ATCURL != "" {
		client = rc.NewClient(command.ATCURL, command.Insecure)
	} else {
		client, err = rc.CommandTargetClient(Fly.Target, &command.Insecure)
	}
	if err != nil {
		return err
	}

	authMethods, err := client.ListAuthMethods()
	if err != nil {
		return err
	}

	var chosenMethod atc.AuthMethod
	if command.Username != "" && command.Password != "" {
		for _, method := range authMethods {
			if method.Type == atc.AuthTypeBasic {
				chosenMethod = method
				break
			}
		}

		if chosenMethod.Type == "" {
			return errors.New("basic auth is not available")
		}
	} else if command.Token != "" {
		oauths := []atc.AuthMethod{}

		for _, method := range authMethods {
			if method.Type == atc.AuthTypeOAuth {
				oauths = append(oauths, method)
			}
		}

		switch len(oauths) {
		case 0:
			return errors.New("oauth is not available")
		case 1:
			chosenMethod = oauths[0]
		}
	}

	if chosenMethod.Type == "" {
		switch len(authMethods) {
		case 0:
			return command.saveTarget(
				client.URL(),
				&rc.TargetToken{},
			)
		case 1:
			chosenMethod = authMethods[0]
		default:
			choices := make([]interact.Choice, len(authMethods))
			for i, method := range authMethods {
				choices[i] = interact.Choice{
					Display: method.DisplayName,
					Value:   method,
				}
			}

			err = interact.NewInteraction("choose an auth method", choices...).Resolve(&chosenMethod)
			if err != nil {
				return err
			}
		}
	}

	return command.loginWith(chosenMethod, client)
}

func (command *LoginCommand) loginWith(method atc.AuthMethod, client concourse.Client) error {
	var token atc.AuthToken

	switch method.Type {
	case atc.AuthTypeOAuth:
		var triedToken bool
		for {
			var tokenStr string

			if command.Token != "" && !triedToken {
				tokenStr = command.Token
				triedToken = true
			} else {
				fmt.Println("navigate to the following URL in your browser:")
				fmt.Println("")
				fmt.Printf("    %s\n", method.AuthURL)
				fmt.Println("")

				err := interact.NewInteraction("enter token").Resolve(interact.Required(&tokenStr))
				if err != nil {
					return err
				}
			}

			segments := strings.SplitN(tokenStr, " ", 2)
			if len(segments) != 2 {
				fmt.Println("token must be of the format 'TYPE VALUE', e.g. 'Bearer ...'")
				continue
			}

			token.Type = segments[0]
			token.Value = segments[1]

			break
		}

	case atc.AuthTypeBasic:
		var username string
		if command.Username != "" {
			username = command.Username
		} else {
			err := interact.NewInteraction("username").Resolve(interact.Required(&username))
			if err != nil {
				return err
			}
		}

		var password string
		if command.Password != "" {
			password = command.Password
		} else {
			var interactivePassword interact.Password
			err := interact.NewInteraction("password").Resolve(interact.Required(&interactivePassword))
			if err != nil {
				return err
			}
			password = string(interactivePassword)
		}

		newUnauthedClient := rc.NewClient(client.URL(), command.Insecure)

		basicAuthClient := concourse.NewClient(
			newUnauthedClient.URL(),
			&http.Client{
				Transport: basicAuthTransport{
					username: username,
					password: password,
					base:     newUnauthedClient.HTTPClient().Transport,
				},
			},
		)

		var err error
		token, err = basicAuthClient.AuthToken()
		if err != nil {
			return err
		}
	}

	return command.saveTarget(
		client.URL(),
		&rc.TargetToken{
			Type:  token.Type,
			Value: token.Value,
		},
	)
}

func (command *LoginCommand) saveTarget(url string, token *rc.TargetToken) error {
	err := rc.SaveTarget(
		Fly.Target,
		url,
		command.Insecure,
		&rc.TargetToken{
			Type:  token.Type,
			Value: token.Value,
		},
	)
	if err != nil {
		return err
	}

	fmt.Println("target saved")

	return nil
}

type basicAuthTransport struct {
	username string
	password string

	base http.RoundTripper
}

func (t basicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.SetBasicAuth(t.username, t.password)
	return t.base.RoundTrip(r)
}

func isURL(passedURL string) bool {
	matched, _ := regexp.MatchString("^http[s]?://", passedURL)
	return matched
}
