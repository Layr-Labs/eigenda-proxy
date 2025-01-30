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

/*
	TODO: https://github.com/Layr-Labs/eigenda-proxy/issues/268

	This CLI logic is already defined in the eigenda monorepo:
	 https://github.com/Layr-Labs/eigenda/blob/0d293cc031987c43f653535732c6e1f1fa65a0b2/common/logger_config.go

	This regression is due to the fact the proxy leverage urfave/cli/v2 whereas
	core eigenda predominantly uses urfave/cli (i.e, v1).

*/

const (
	PathFlagName   = "path"
	LevelFlagName  = "level"
	FormatFlagName = "format"
	// deprecated
	PidFlagName   = "pid"
	ColorFlagName = "color"
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
	category := "logging"

	return []cli.Flag{
		&cli.StringFlag{
			Name:     common.PrefixFlag(flagPrefix, LevelFlagName),
			Category: category,
			Usage:    `The lowest log level that will be output. Accepted options are "debug", "info", "warn", "error"`,
			Value:    "info",
			EnvVars:  []string{common.PrefixEnvVar(envPrefix, "LOG_LEVEL")},
		},
		&cli.StringFlag{
			Name:     common.PrefixFlag(flagPrefix, PathFlagName),
			Category: category,
			Usage:    "Path to file where logs will be written",
			Value:    "",
			EnvVars:  []string{common.PrefixEnvVar(envPrefix, "LOG_PATH")},
		},
		&cli.StringFlag{
			Name:     common.PrefixFlag(flagPrefix, FormatFlagName),
			Category: category,
			Usage:    "The format of the log file. Accepted options are 'json' and 'text'",
			Value:    "json",
			EnvVars:  []string{common.PrefixEnvVar(envPrefix, "LOG_FORMAT")},
		},

		// Deprecated since used by op-service logging which has been replaced
		// by eigengo-sdk logger
		&cli.BoolFlag{
			Name:     common.PrefixFlag(flagPrefix, PidFlagName),
			Category: category,
			Usage:    "Show pid in the log",
			EnvVars:  []string{common.PrefixEnvVar(envPrefix, "LOG_PID")},
			Hidden:   true,
			Action: func(_ *cli.Context, _ bool) error {
				return fmt.Errorf("flag --%s is deprecated", PidFlagName)
			},
		},

		&cli.BoolFlag{
			Name:     common.PrefixFlag(flagPrefix, ColorFlagName),
			Category: category,
			Usage:    "Color the log output if in terminal mode",
			EnvVars:  []string{common.PrefixEnvVar(envPrefix, "LOG_COLOR")},
			Hidden:   true,
			Action: func(_ *cli.Context, _ bool) error {
				return fmt.Errorf("flag --%s is deprecated", ColorFlagName)
			},
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
