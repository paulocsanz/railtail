package config

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/half0wl/railtail/internal/config/parser"
)

type ForwardTrafficType string

const (
	ForwardTrafficTypeTCP   ForwardTrafficType = "tcp"
	ForwardTrafficTypeHTTP  ForwardTrafficType = "http"
	ForwardTrafficTypeHTTPS ForwardTrafficType = "https"
)

var (
	ErrTargetAddrInvalid = errors.New("target-addr is invalid")
)

type Mode string

const (
	// ModeForward is the existing behavior: a tailnet-facing listener
	// forwarding to one fixed Railway-private TargetAddr (ts.Listen +
	// plain net.Dial). Used for the federation-api bridge.
	ModeForward Mode = "forward"
	// ModeSocks5 (RFC 0101 P1-5): the opposite direction — a plain
	// Railway-private-facing listener (net.Listen) serving a generic
	// SOCKS5 proxy backed by ts.Dial, so a caller in Railway's private
	// network (procurador) can reach ARBITRARY tailnet addresses per
	// request instead of one fixed target. No TargetAddr in this mode —
	// the SOCKS5 protocol itself carries the destination per-connection.
	ModeSocks5 Mode = "socks5"
)

type Config struct {
	TSHostname string `flag:"ts-hostname" env:"TS_HOSTNAME" usage:"hostname to use for tailscale"`
	ListenPort string `flag:"listen-port" env:"LISTEN_PORT" usage:"port to listen on"`
	// default:"" makes this optional at the parser level — required only
	// in ModeForward, enforced below in LoadConfig, not here. ModeSocks5
	// doesn't use it at all.
	TargetAddr     string `flag:"target-addr" env:"TARGET_ADDR" default:"" usage:"address:port of a tailscale node to send traffic to (ModeForward only)"`
	TSLoginServer  string `flag:"ts-login-server" env:"TS_LOGIN_SERVER" default:"" usage:"base url of the control server, If you are using Headscale for your control server, use your Headscale instance's URL"`
	TSStateDirPath string `flag:"ts-state-dir" env:"TS_STATEDIR_PATH" default:"/tmp/railtail" usage:"tailscale state dir"`
	TSAuthKey      string `env:"TS_AUTHKEY,TS_AUTH_KEY" usage:"tailscale auth key"`
	RailtailMode   string `flag:"mode" env:"RAILTAIL_MODE" default:"forward" usage:"forward (default, existing behavior) or socks5 (RFC 0101 P1-5)"`

	ForwardTrafficType ForwardTrafficType
	Mode               Mode
}

func init() {
	// add help flag purely for the usage message
	flag.Bool("help", false, "Show help message")

	// Only parse and print usage if -help is present in arguments
	if checkForFlag("help") {
		// Create temporary config just to register all flags for usage message
		cfg := &Config{}

		parser.ParseFlags(cfg)

		flag.Usage()
		os.Exit(0)
	}
}

func LoadConfig() (*Config, []error) {
	cfg := &Config{}

	errors := parser.ParseConfig(cfg)

	switch Mode(cfg.RailtailMode) {
	case ModeSocks5:
		cfg.Mode = ModeSocks5
		if cfg.TargetAddr != "" {
			errors = append(errors, fmt.Errorf("target-addr must not be set in socks5 mode (RAILTAIL_MODE=socks5) — the socks5 protocol carries the destination per-connection"))
		}
	case ModeForward, "":
		cfg.Mode = ModeForward
		if cfg.TargetAddr == "" {
			errors = append(errors, fmt.Errorf("target-addr is required in forward mode: set TARGET_ADDR in env or use --target-addr"))
		}
	default:
		errors = append(errors, fmt.Errorf("invalid mode %q: expected \"forward\" or \"socks5\"", cfg.RailtailMode))
	}

	// Validate target-addr if it's set to either be a valid URL with a port or a valid address:port
	if cfg.TargetAddr != "" {
		protocol := strings.SplitN(cfg.TargetAddr, "://", 2)[0]

		switch protocol {
		case "https", "http":
			cfg.ForwardTrafficType = ForwardTrafficType(protocol)

			u, err := url.Parse(cfg.TargetAddr)
			if err != nil {
				errors = append(errors, fmt.Errorf("%w: %w", ErrTargetAddrInvalid, err))
			}

			// Check if the URL has a port only if the URL is valid
			if err == nil && u.Port() == "" {
				errors = append(errors, fmt.Errorf("%w: address %s: missing port in address", ErrTargetAddrInvalid, cfg.TargetAddr))
			}
		default:
			cfg.ForwardTrafficType = ForwardTrafficTypeTCP

			_, _, err := net.SplitHostPort(cfg.TargetAddr)
			if err != nil {
				errors = append(errors, fmt.Errorf("%w: %w", ErrTargetAddrInvalid, err))
			}
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return cfg, nil
}
