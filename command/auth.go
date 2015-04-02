package command

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/vault/helper/password"
	"github.com/ryanuber/columnize"
)

// AuthCommand is a Command that handles authentication.
type AuthCommand struct {
	Meta
}

func (c *AuthCommand) Run(args []string) int {
	var method string
	var methods bool
	flags := c.Meta.FlagSet("auth", FlagSetDefault)
	flags.BoolVar(&methods, "methods", false, "")
	flags.StringVar(&method, "method", "", "method")
	flags.Usage = func() { c.Ui.Error(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if methods {
		return c.listMethods()
	}

	args = flags.Args()
	if len(args) > 1 {
		flags.Usage()
		c.Ui.Error("\nError: auth expects at most one argument")
		return 1
	}
	if method != "" && len(args) > 0 {
		flags.Usage()
		c.Ui.Error("\nError: auth expects no arguments if -method is specified")
		return 1
	}

	tokenHelper, err := c.TokenHelper()
	if err != nil {
		c.Ui.Error(fmt.Sprintf(
			"Error initializing token helper: %s\n\n"+
				"Please verify that the token helper is available and properly\n"+
				"configured for your system. Please refer to the documentation\n"+
				"on token helpers for more information.",
			err))
		return 1
	}

	// token is where the final token will go
	var token string
	if method == "" {
		if len(args) > 0 {
			token = args[0]

			// TODO(mitchellh): stdin
		} else {
			// No arguments given, read the token from user input
			fmt.Printf("Token (will be hidden): ")
			token, err = password.Read(os.Stdin)
			fmt.Printf("\n")
			if err != nil {
				c.Ui.Error(fmt.Sprintf(
					"Error attempting to ask for token. The raw error message\n"+
						"is shown below, but the most common reason for this error is\n"+
						"that you attempted to pipe a value into auth. If you want to\n"+
						"pipe the token, please pass '-' as the token argument.\n\n"+
						"Raw error: %s", err))
				return 1
			}
		}

		if token == "" {
			c.Ui.Error(fmt.Sprintf(
				"A token must be passed to auth. Please view the help\n" +
					"for more information."))
			return 1
		}
	} else {
		// TODO(mitchellh): other auth methods
	}

	// Store the token!
	if err := tokenHelper.Store(token); err != nil {
		c.Ui.Error(fmt.Sprintf(
			"Error storing token: %s\n\n"+
				"Authentication was not successful and did not persist.\n"+
				"Please reauthenticate, or fix the issue above if possible.",
			err))
		return 1
	}

	// Build the client so we can verify that the token is valid
	client, err := c.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf(
			"Error initializing client to verify the token: %s", err))
		return 1
	}

	// Verify the token
	secret, err := client.Logical().Read("auth/token/lookup-self")
	if err != nil {
		c.Ui.Error(fmt.Sprintf(
			"Error validating token: %s", err))
		return 1
	}

	// Get the policies we have
	policiesRaw, ok := secret.Data["policies"]
	if !ok {
		policiesRaw = []string{"unknown"}
	}
	var policies []string
	for _, v := range policiesRaw.([]interface{}) {
		policies = append(policies, v.(string))
	}

	c.Ui.Output(fmt.Sprintf(
		"Successfully authenticated! The policies that are associated\n"+
			"with this token are listed below:\n\n%s",
		strings.Join(policies, ", "),
	))

	return 0
}

func (c *AuthCommand) listMethods() int {
	client, err := c.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf(
			"Error initializing client: %s", err))
		return 1
	}

	auth, err := client.Sys().ListAuth()
	if err != nil {
		c.Ui.Error(fmt.Sprintf(
			"Error reading auth table: %s", err))
		return 1
	}

	paths := make([]string, 0, len(auth))
	for path, _ := range auth {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	columns := []string{"Path | Type | Description"}
	for _, k := range paths {
		a := auth[k]
		columns = append(columns, fmt.Sprintf(
			"%s | %s | %s", k, a.Type, a.Description))
	}

	c.Ui.Output(columnize.SimpleFormat(columns))
	return 0
}

func (c *AuthCommand) Synopsis() string {
	return "Prints information about how to authenticate with Vault"
}

func (c *AuthCommand) Help() string {
	helpText := `
Usage: vault auth [options] [token]

  Authenticate with Vault with the given token or via any supported
  authentication backend.

  If no -method is specified, then the token is expected. If it is not
  given on the command-line, it will be asked via user input. If the
  token is "-", it will be read from stdin.

  By specifying -method, alternate authentication methods can be used
  such as OAuth or TLS certificates. For these, additional -var flags
  may be required. It is an error to specify a token with -method.

General Options:

  -address=TODO           The address of the Vault server.

  -ca-cert=path           Path to a PEM encoded CA cert file to use to
                          verify the Vault server SSL certificate.

  -ca-path=path           Path to a directory of PEM encoded CA cert files
                          to verify the Vault server SSL certificate. If both
                          -ca-cert and -ca-path are specified, -ca-path is used.

  -insecure               Do not verify TLS certificate. This is highly
                          not recommended.

Auth Options:

  -method=name    Outputs help for the authentication method with the given
                  name for the remote server. If this authentication method
                  is not available, exit with code 1.

  -methods        List the available auth methods.

`
	return strings.TrimSpace(helpText)
}
