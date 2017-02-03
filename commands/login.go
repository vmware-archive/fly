package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"net"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type LoginCommand struct {
	ATCURL        string       `short:"c" long:"concourse-url" description:"Concourse URL to authenticate with"`
	Insecure      bool         `short:"k" long:"insecure" description:"Skip verification of the endpoint's SSL certificate"`
	Username      string       `short:"u" long:"username" description:"Username for basic auth"`
	Password      string       `short:"p" long:"password" description:"Password for basic auth"`
	TeamName      string       `short:"n" long:"team-name" description:"Team to authenticate with" default:"main"`
	CACert        atc.PathFlag `long:"ca-cert" description:"Path to Concourse PEM-encoded CA certificate file."`
	ClientCert    atc.PathFlag `long:"client-cert" description:"Path to Concourse PEM-encoded client certificate file."`
	ClientCertKey atc.PathFlag `long:"client-cert-key" description:"Path to Concourse PEM-encoded client certificate private key file."`
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

	var clientCert string
	var clientCertKey string
	if (command.ClientCertKey != "" && command.ClientCert == "") || (command.ClientCertKey == "" && command.ClientCert != "") {
		return errors.New("--client-cert and --client-cert-key must be both specified if client cert is used")
	}
	if command.ClientCert != "" {
		clientCertBytes, err := ioutil.ReadFile(string(command.ClientCert))
		if err != nil {
			return err
		}
		clientCertKeyBytes, err := ioutil.ReadFile(string(command.ClientCertKey))
		if err != nil {
			return err
		}
		clientCert = string(clientCertBytes)
		clientCertKey = string(clientCertKeyBytes)
	}

	if command.ATCURL != "" {
		target, err = rc.NewUnauthenticatedTarget(
			Fly.Target,
			command.ATCURL,
			command.TeamName,
			command.Insecure,
			caCert,
			clientCert,
			clientCertKey,
		)
	} else {
		target, err = rc.LoadTargetWithInsecure(
			Fly.Target,
			command.TeamName,
			command.Insecure,
			caCert,
			clientCert,
			clientCertKey,
		)
	}
	if err != nil {
		return err
	}

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
				target.ClientCert(),
				target.ClientCertKey(),
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
				target.ClientCert(),
				target.ClientCertKey(),
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

	return command.saveTarget(
		client.URL(),
		&rc.TargetToken{
			Type:  token.Type,
			Value: token.Value,
		},
		target.CACert(),
		target.ClientCert(),
		target.ClientCertKey(),
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

		stdinChannel := make(chan string)
		tokenChannel := make(chan string)
		errorChannel := make(chan error)
		portChannel := make(chan string)

		go listenForTokenCallback(tokenChannel, errorChannel, portChannel, targetUrl)

		port := <-portChannel

		fmt.Println("navigate to the following URL in your browser:")
		fmt.Println("")
		fmt.Printf("    %s&fly_local_port=%s\n", method.AuthURL, port)
		fmt.Println("")

		go waitForTokenInput(stdinChannel, errorChannel)

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

		var clientCert string
		var clientCertKey string
		if (command.ClientCertKey != "" && command.ClientCert == "") || (command.ClientCertKey == "" && command.ClientCert != "") {
			return nil, errors.New("--client-cert and --client-cert-key must be both specified if client cert is used")
		}
		if command.ClientCert != "" {
			clientCertBytes, err := ioutil.ReadFile(string(command.ClientCert))
			if err != nil {
				return nil, err
			}
			clientCertKeyBytes, err := ioutil.ReadFile(string(command.ClientCertKey))
			if err != nil {
				return nil, err
			}
			clientCert = string(clientCertBytes)
			clientCertKey = string(clientCertKeyBytes)
		}

		target, err := rc.NewBasicAuthTarget(
			Fly.Target,
			client.URL(),
			command.TeamName,
			command.Insecure,
			username,
			password,
			caCert,
			clientCert,
			clientCertKey,
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

func waitForTokenInput(tokenChannel chan string, errorChannel chan error) {
	for {
		fmt.Printf("or enter token manually: ")

		var tokenType string
		var tokenValue string
		count, err := fmt.Scanf("%s %s", &tokenType, &tokenValue)
		if err != nil {
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

func (command *LoginCommand) saveTarget(url string, token *rc.TargetToken, caCert string, clientCert string, clientCertKey string) error {
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
		clientCert,
		clientCertKey,
	)
	if err != nil {
		return err
	}

	fmt.Println("")
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
