package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"net"

	"crypto/tls"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/pkg/browser"
	"github.com/vito/go-interact/interact"
)

type LoginCommand struct {
	ATCURL    string       `short:"c" long:"concourse-url" description:"Concourse URL to authenticate with"`
	Insecure  bool         `short:"k" long:"insecure" description:"Skip verification of the endpoint's SSL certificate"`
	Username  string       `short:"u" long:"username" description:"Username for basic auth"`
	Password  string       `short:"p" long:"password" description:"Password for basic auth"`
	TeamName  string       `short:"n" long:"team-name" description:"Team to authenticate with"`
	Token     string       `long:"token" description:"Token for OAuth login"`
	CACert    atc.PathFlag `long:"ca-cert" description:"Path to Concourse PEM-encoded CA certificate file."`
	NoBrowser bool         `long:"no-browser" description:"Print URL, but don't attempt launch a browser for login"`
}

func (command *LoginCommand) Execute(args []string) error {
	if Fly.Target == "" {
		return errors.New("name for the target must be specified (--target/-t)")
	}

	var target rc.Target
	var err error

	var caCert string
	if command.CACert != "" {
		caCertBytes, err := ioutil.ReadFile(string(command.CACert))
		if err != nil {
			return err
		}
		caCert = string(caCertBytes)
	}

	if command.ATCURL != "" {
		if command.TeamName == "" {
			command.TeamName = atc.DefaultTeamName
		}

		target, err = rc.NewUnauthenticatedTarget(
			Fly.Target,
			command.ATCURL,
			command.TeamName,
			command.Insecure,
			caCert,
			Fly.Verbose,
		)
	} else {
		target, err = rc.LoadTargetWithInsecure(
			Fly.Target,
			command.TeamName,
			command.Insecure,
			caCert,
			Fly.Verbose,
		)
	}
	if err != nil {
		return err
	}

	command.TeamName = target.Team().Name()

	fmt.Printf("logging in to team '%s'\n\n", command.TeamName)

	err = target.ValidateWithWarningOnly()
	if err != nil {
		return err
	}

	authMethods, err := target.Team().ListAuthMethods()
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
	} else {
		switch len(authMethods) {
		case 0:
			target, err := rc.NewNoAuthTarget(
				Fly.Target,
				target.Client().URL(),
				command.TeamName,
				command.Insecure,
				target.CACert(),
				Fly.Verbose,
			)
			if err != nil {
				return err
			}

			token, err := target.Team().AuthToken()
			if err != nil {
				return err
			}

			return command.saveTarget(
				target.Client().URL(),
				&rc.TargetToken{
					Type:  token.Type,
					Value: token.Value,
				},
				target.CACert(),
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

	client := target.Client()
	token, err := command.loginWith(chosenMethod, client, caCert, target.Client().URL())
	if err != nil {
		return err
	}

	fmt.Println("")

	return command.saveTarget(
		client.URL(),
		&rc.TargetToken{
			Type:  token.Type,
			Value: token.Value,
		},
		target.CACert(),
	)
}

func listenForTokenCallback(tokenChannel chan string, errorChannel chan error, portChannel chan string, targetUrl string) {
	s := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenChannel <- r.FormValue("token")
			http.Redirect(w, r, fmt.Sprintf("%s/public/fly_success", targetUrl), http.StatusTemporaryRedirect)
		}),
	}

	err := listenAndServeWithPort(s, portChannel)

	if err != nil {
		errorChannel <- err
	}
}

func (command *LoginCommand) loginWith(
	method atc.AuthMethod,
	client concourse.Client,
	caCert string,
	targetUrl string,
) (*atc.AuthToken, error) {
	var token atc.AuthToken

	switch method.Type {
	case atc.AuthTypeOAuth:
		var tokenStr string

		if command.Token != "" {
			fmt.Println("Yeah, who needs the web auth flow anyway? Token FTW!")

			tradedToken, err := usePersonalToken(method.TokenURL, command.Token, command.Insecure)

			if err != nil {
				return nil, err
			}

			segments := strings.SplitN(tradedToken, " ", 2)

			token.Type = segments[0]
			token.Value = segments[1]

		} else {
			stdinChannel := make(chan string)
			tokenChannel := make(chan string)
			errorChannel := make(chan error)
			portChannel := make(chan string)

			go listenForTokenCallback(tokenChannel, errorChannel, portChannel, targetUrl)

			port := <-portChannel

			urlForAcceptance := fmt.Sprintf("%s&fly_local_port=%s", method.AuthURL, port)
			fmt.Println("navigate to the following URL in your browser:")
			fmt.Println("")
			fmt.Printf("    %s\n", urlForAcceptance)
			fmt.Println("")

			if !command.NoBrowser {
				browser.OpenURL(urlForAcceptance) // ignore error code, we don't care if this fails
			}

			go waitForTokenInput(stdinChannel, errorChannel, method.TokenURL, command.Insecure)

			select {
			case tokenStrMsg := <-tokenChannel:
				tokenStr = tokenStrMsg
			case tokenStrMsg := <-stdinChannel:
				tokenStr = tokenStrMsg
			case errorMsg := <-errorChannel:
				return nil, errorMsg
			}
			segments := strings.SplitN(tokenStr, " ", 2)

			token.Type = segments[0]
			token.Value = segments[1]
		}

	case atc.AuthTypeBasic:
		var username string
		if command.Username != "" {
			username = command.Username
		} else {
			err := interact.NewInteraction("username").Resolve(interact.Required(&username))
			if err != nil {
				return nil, err
			}
		}

		var password string
		if command.Password != "" {
			password = command.Password
		} else {
			var interactivePassword interact.Password
			err := interact.NewInteraction("password").Resolve(interact.Required(&interactivePassword))
			if err != nil {
				return nil, err
			}
			password = string(interactivePassword)
		}

		target, err := rc.NewBasicAuthTarget(
			Fly.Target,
			client.URL(),
			command.TeamName,
			command.Insecure,
			username,
			password,
			caCert,
			Fly.Verbose,
		)
		if err != nil {
			return nil, err
		}

		token, err = target.Team().AuthToken()
		if err != nil {
			return nil, err
		}
	}

	return &token, nil
}

func waitForTokenInput(tokenChannel chan string, errorChannel chan error, tokenUrl string, insecure bool) {
	for {
		fmt.Printf("or enter one of the following token types:\n")
		fmt.Printf("    - Personal access token (e.g. '1234567890')\n")
		fmt.Printf("    - Bearer token (e.g. 'Bearer 1234567890')\n\n")
		fmt.Printf("Token: ")

		var tokenType string
		var tokenValue string
		count, err := fmt.Scanf("%s %s", &tokenType, &tokenValue)
		if err != nil {
			if count == 1 {
				// assume it is a token...
				tradedToken, err := usePersonalToken(tokenUrl, tokenType, insecure)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				tokenChannel <- tradedToken
				break
			}
			if count != 2 {
				fmt.Println("token must be of the format 'TYPE VALUE', e.g. 'Bearer ...'")
				continue
			}

			errorChannel <- err
			return
		}

		tokenChannel <- tokenType + " " + tokenValue
		break
	}
}

func usePersonalToken(tokenUrl string, token string, insecure bool) (string, error) {
	httpTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}
	client := &http.Client{Transport: httpTransport}

	response, err := client.Post(tokenUrl, "text/plain", strings.NewReader(token))

	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	if response.StatusCode != 200 {
		return "", errors.New(string(body))
	}
	return string(body), nil
}

func (command *LoginCommand) saveTarget(url string, token *rc.TargetToken, caCert string) error {
	err := rc.SaveTarget(
		Fly.Target,
		url,
		command.Insecure,
		command.TeamName,
		&rc.TargetToken{
			Type:  token.Type,
			Value: token.Value,
		},
		caCert,
	)
	if err != nil {
		return err
	}

	fmt.Println("target saved")

	return nil
}

func listenAndServeWithPort(srv *http.Server, portChannel chan string) error {
	addr := srv.Addr
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	_, port, err := net.SplitHostPort(ln.Addr().String())

	portChannel <- port

	return srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}
