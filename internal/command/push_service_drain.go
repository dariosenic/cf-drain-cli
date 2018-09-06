package command

import (
	"fmt"

	"code.cloudfoundry.org/cli/plugin"
	flags "github.com/jessevdk/go-flags"
)

type pushServiceDrainOpts struct {
	DrainName string
	DrainURL  string
	Path      string `long:"path"`
}

type GroupNameProvider func() string
type GUIDProvider func() string

func PushServiceDrain(
	cli plugin.CliConnection,
	args []string,
	f RefreshTokenFetcher,
	log Logger,
	g GroupNameProvider,
) {
	var opts pushServiceDrainOpts
	parser := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	args, err := parser.ParseArgs(args)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if len(args) != 2 {
		log.Fatalf("Invalid arguments, expected 2 got %d.", len(args))
	}

	service, err := cli.GetService(args[0])
	if err != nil {
		log.Fatalf("%s", err)
	}

	skipCertVerify, err := cli.IsSSLDisabled()
	if err != nil {
		log.Fatalf("%s", err)
	}

	refreshToken, err := f.RefreshToken()
	if err != nil {
		log.Fatalf("%s", err)
	}

	space, err := cli.GetCurrentSpace()
	if err != nil {
		log.Fatalf("%s", err)
	}

	org, err := cli.GetCurrentOrg()
	if err != nil {
		log.Fatalf("%s", err)
	}

	opts.DrainName = fmt.Sprintf("%s-forwarder", service.Name)
	opts.DrainURL = args[1]

	envs := [][]string{
		{"SOURCE_ID", service.Guid},
		{"SOURCE_HOSTNAME", fmt.Sprintf("%s.%s.%s", org.Name, space.Name, opts.DrainName)},
		{"CLIENT_ID", "cf"},
		{"REFRESH_TOKEN", refreshToken},
		{"CACHE_SIZE", "0"},
		{"SKIP_CERT_VERIFY", fmt.Sprintf("%t", skipCertVerify)},
		{"GROUP_NAME", g()},
		{"SYSLOG_URL", opts.DrainURL},
	}
	pushSyslogForwarder(cli, log, opts.DrainName, opts.Path, envs)
}

func pushSyslogForwarder(cli plugin.CliConnection, log Logger, drainName, path string, envs [][]string) {
	_, err := cli.CliCommand(
		"push", drainName,
		"-p", path,
		"-i", "3",
		"-b", "binary_buildpack",
		"-c", "./run.sh",
		"--health-check-type", "process",
		"--no-start",
		"--no-route",
	)
	if err != nil {
		log.Fatalf("%s", err)
	}

	for _, env := range envs {
		_, err := cli.CliCommandWithoutTerminalOutput("set-env", drainName, env[0], env[1])
		if err != nil {
			log.Fatalf("%s", err)
		}
	}

	cli.CliCommand("start", drainName)
}
