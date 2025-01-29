package common

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Layr-Labs/eigenda/common"
	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/urfave/cli/v2"
)

const (
	PathFlagName   = "log.path"
	LevelFlagName  = "log.level"
	FormatFlagName = "log.format"
)

type LogFormat string

const (
	JSONLogFormat LogFormat = "json"
	TextLogFormat LogFormat = "text"
)

type LoggerConfig struct {
	Format       LogFormat
	OutputWriter io.Writer
	HandlerOpts  logging.SLoggerOptions
}

func LoggerCLIFlags(envPrefix string, flagPrefix string) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    common.PrefixFlag(flagPrefix, LevelFlagName),
			Usage:   `The lowest log level that will be output. Accepted options are "debug", "info", "warn", "error"`,
			Value:   "info",
			EnvVars: []string{common.PrefixEnvVar(envPrefix, "LOG_LEVEL")},
		},
		&cli.StringFlag{
			Name:    common.PrefixFlag(flagPrefix, PathFlagName),
			Usage:   "Path to file where logs will be written",
			Value:   "",
			EnvVars: []string{common.PrefixEnvVar(envPrefix, "LOG_PATH")},
		},
		&cli.StringFlag{
			Name:    common.PrefixFlag(flagPrefix, FormatFlagName),
			Usage:   "The format of the log file. Accepted options are 'json' and 'text'",
			Value:   "json",
			EnvVars: []string{common.PrefixEnvVar(envPrefix, "LOG_FORMAT")},
		},
	}
}

func ReadLoggerCLIConfig(ctx *cli.Context, flagPrefix string) (*common.LoggerConfig, error) {
	cfg := common.DefaultLoggerConfig()
	format := ctx.String(common.PrefixFlag(flagPrefix, FormatFlagName))
	if format == "json" {
		cfg.Format = common.JSONLogFormat
	} else if format == "text" {
		cfg.Format = common.TextLogFormat
	} else {
		return nil, fmt.Errorf("invalid log file format %s", format)
	}

	path := ctx.String(common.PrefixFlag(flagPrefix, PathFlagName))
	if path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		cfg.OutputWriter = io.MultiWriter(os.Stdout, f)
	}
	logLevel := ctx.String(common.PrefixFlag(flagPrefix, LevelFlagName))
	var level slog.Level
	err := level.UnmarshalText([]byte(logLevel))
	if err != nil {
		panic("failed to parse log level " + logLevel)
	}
	cfg.HandlerOpts.Level = level

	return &cfg, nil
}
