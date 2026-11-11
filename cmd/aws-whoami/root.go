package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/cobra"

	awswhoami "github.com/scottbrown/go-aws-whoami"
)

var version = "dev"

const description = `Show what AWS account and identity you're using.
Formats the output of sts.GetCallerIdentity nicely,
and also gets your account alias (if you're allowed)`

type Profile string

type whoamiFunc func(ctx context.Context, profile Profile, policy awswhoami.AliasPolicy) (awswhoami.Info, error)

type options struct {
	profile string
	json    bool
	debug   bool
}

func awsWhoami(ctx context.Context, profile Profile, policy awswhoami.AliasPolicy) (awswhoami.Info, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(string(profile)))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return awswhoami.Info{}, err
	}
	return awswhoami.Whoami(ctx, cfg, policy)
}

func newRootCmd(whoami whoamiFunc, getenv func(string) string, opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "aws-whoami",
		Short:         "Show what AWS account and identity you're using",
		Long:          description,
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			policy := awswhoami.ParseAliasPolicy(getenv(awswhoami.DisableAccountAliasEnvVar))
			info, err := whoami(cmd.Context(), Profile(opts.profile), policy)
			if err != nil {
				return err
			}
			return render(cmd.OutOrStdout(), info, opts.json)
		},
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.Flags().StringVar(&opts.profile, "profile", "", "AWS profile to use")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&opts.debug, "debug", false, "Print the full error chain on failure")
	return cmd
}

func render(w io.Writer, info awswhoami.Info, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(info)
	}
	_, err := fmt.Fprintln(w, awswhoami.Format(info))
	return err
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, whoami whoamiFunc, getenv func(string) string) int {
	opts := &options{}
	cmd := newRootCmd(whoami, getenv, opts)
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	if opts.debug {
		writeErrorChain(stderr, err)
	}
	fmt.Fprintf(stderr, "ERROR [%T]: %v\n", err, err)
	return 1
}

func writeErrorChain(w io.Writer, err error) {
	for depth := 0; err != nil; depth++ {
		fmt.Fprintf(w, "%*s%T: %v\n", depth*2, "", err, err)
		err = errors.Unwrap(err)
	}
}
