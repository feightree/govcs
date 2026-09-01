package v16

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	v16 "github.com/feightree/gocpp/v16"
)

type createFlags struct {
	id                string
	csms              string
	vendor            string
	model             string
	iccid             string
	imsi              string
	heartbeatInterval int
	connectors        int
}

var (
	// Usage errors, should print help menu
	ErrRootUsage   = errors.New("incorrect use")
	ErrCreateUsage = errors.New("incorrect use of create command")
	ErrStartUsage  = errors.New("incorrect use of start command")
	// Actual errors happening
	ErrSaveFile        = errors.New("failed to save state file")
	ErrLoadFile        = errors.New("failed to load state file")
	ErrInvalidFilePath = errors.New("invalid FILE_PATH")
)

type usageError struct {
	kind error
	err  error
}

func (e *usageError) Error() string        { return e.err.Error() }
func (e *usageError) Unwrap() error        { return e.err }
func (e *usageError) Is(target error) bool { return target == e.kind }

func newUsageError(kind, err error) error {
	return &usageError{
		kind: kind,
		err:  err,
	}
}

func Dispatch(bin string, args []string) error {
	if len(args) < 2 || args[1] == "help" || args[1] == "-h" || args[1] == "--help" {
		PrintRootUsage(bin)
		return nil
	}

	switch args[1] {
	case "create":
		if len(args) < 3 || strings.HasPrefix(args[2], "-") {
			return newUsageError(ErrCreateUsage, ErrInvalidFilePath)
		}

		path := args[2]
		fs := flag.NewFlagSet("create", flag.ExitOnError)
		opts, err := ParseCreateFlags(fs, args[3:])
		if err != nil {
			return newUsageError(ErrCreateUsage, err)
		}

		cp := ChargePoint{
			ID:                opts.id,
			CSMS:              opts.csms,
			Vendor:            opts.vendor,
			Model:             opts.model,
			IMSI:              opts.imsi,
			ICCID:             opts.iccid,
			HeartbeatInterval: int32(opts.heartbeatInterval),
			Connectors:        []Connector{},
		}

		for i := range opts.connectors {
			cp.Connectors = append(cp.Connectors, Connector{
				ID:          int32(i + 1),
				Status:      v16.ChargePointStatusAvailable,
				ErrorCode:   v16.ChargePointErrorCodeNoError,
				Transaction: nil,
			})
		}

		if err := Save(&cp, path); err != nil {
			return fmt.Errorf("%w: %w", ErrSaveFile, err)
		}
		return nil

	case "start":
		if len(args) < 3 {
			PrintStartUsage(bin)
			return nil
		}

		if strings.HasPrefix(args[2], "-") {
			return newUsageError(ErrStartUsage, ErrInvalidFilePath)
		}

		path := args[2]
		_, err := Load(path)

		if err != nil {
			return fmt.Errorf("%w: %w", ErrLoadFile, err)
		}

		// Start cp?
		return nil

	default:
		return ErrRootUsage
	}
}

// PrintRootUsage prints the root help menu.
func PrintRootUsage(bin string) {
	fmt.Printf(`
Usage: %s [GLOBAL OPTIONS] [COMMAND] [OPTIONS]

Command:
  create    Create a new charger
  start     Start charger

Global Options:
  -h, --help    Prints this help menu

Run '%s COMMAND --help' for more information on a command.
`, bin, bin)
}

// PrintStartUsage prints the help menu for the `start` command.
func PrintStartUsage(bin string) {
	fmt.Printf(`
Usage: %s start FILE_PATH

Starts up a new chargepoint from a state-file

Run '%s --help' to see all available commands.
`, bin, bin)
}

// PrintCreateUsage prints the help menu for the `create` command.
func PrintCreateUsage(bin string) {
	fmt.Printf(`
Usage: %s create FILE_PATH [OPTIONS]

Creates a new json state-file that can be used to run a charger with a specific
configuration. It's adviced to use the 'id' as the filename.

Example: %s create ./my-chargepoint.json -id my-chargepoint

After creation, it's recommended to review and modify the state-file as needed.

Options:
  -id <string>           (Required) Chargepoint identity
  -csms <url>            (Required) Websocket URL to connect to
  -vendor <string>       Vendor name (default: "govcs")
  -model <string>        Model name (default: "vcs")
  -imsi <string>         International Mobile Subscriber Identity
  -iccid <string>        Integrated Circuit Card Identifier
  -heartbeat <seconds>   Heartbeat interval (default: 60)
  -connectors <int>      Number of connectors to scaffold (default: 2)

Run '%s --help' to see all available commands.
`, bin, bin, bin)
}

func ParseCreateFlags(fs *flag.FlagSet, args []string) (*createFlags, error) {
	opts := createFlags{}
	fs.StringVar(&opts.id, "id", "", "id of the chargepoint")
	fs.StringVar(&opts.csms, "csms", "", "websocket url to connect to")
	fs.StringVar(&opts.vendor, "vendor", "govcs", "chargepoint vendor")
	fs.StringVar(&opts.model, "model", "vcs", "chargepoint model")
	fs.StringVar(&opts.iccid, "iccid", "", "chargepoint ICCID")
	fs.StringVar(&opts.imsi, "imsi", "", "chargepoint IMSI")
	fs.IntVar(&opts.heartbeatInterval, "heartbeat", 60, "heartbeat interval (in seconds)")
	fs.IntVar(&opts.connectors, "connectors", 2, "number of connectors")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parsing error: %w", err)
	}

	if opts.id == "" {
		return nil, fmt.Errorf("missing required flag: id")
	}

	if opts.csms == "" {
		return nil, fmt.Errorf("missing required flag: csms")
	}

	return &opts, nil
}
