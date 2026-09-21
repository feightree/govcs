package v16

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	v16 "github.com/feightree/gocpp/v16"
)

// createFlags holds the parsed command-line flags for the "create"
// command, before they're copied into a ChargePoint.
type createFlags struct {
	id                string
	csmsUrl           string
	serialnumber      string
	vendor            string
	model             string
	iccid             string
	imsi              string
	firmwareVersion   string
	meterType         string
	meterSerialnumber string
	heartbeatInterval int
	connectors        int
}

var (
	// ErrRootUsage indicates the CLI was invoked with no recognized
	// command.
	ErrRootUsage = errors.New("incorrect use")
	// ErrCreateUsage wraps a usageError kind for a malformed "create"
	// invocation - check with errors.Is.
	ErrCreateUsage = errors.New("incorrect use of create command")
	// ErrStartUsage wraps a usageError kind for a malformed "start"
	// invocation - check with errors.Is.
	ErrStartUsage = errors.New("incorrect use of start command")

	// ErrSaveFile indicates a ChargePoint's state-file failed to save.
	ErrSaveFile = errors.New("failed to save state file")
	// ErrLoadFile indicates a ChargePoint's state-file failed to load.
	ErrLoadFile = errors.New("failed to load state file")
	// ErrInvalidFilePath indicates a command's FILE_PATH argument was
	// missing or looked like a flag instead of a path.
	ErrInvalidFilePath = errors.New("invalid FILE_PATH")
)

// usageError pairs an underlying error with a "kind" sentinel (one of the
// Err*Usage errors above), so a caller can match on the general kind via
// errors.Is while Error() still surfaces the specific underlying message.
type usageError struct {
	kind error
	err  error
}

func (e *usageError) Error() string        { return e.err.Error() }
func (e *usageError) Unwrap() error        { return e.err }
func (e *usageError) Is(target error) bool { return target == e.kind }

// newUsageError wraps err as a usageError of the given kind.
func newUsageError(kind, err error) error {
	return &usageError{
		kind: kind,
		err:  err,
	}
}

// Dispatch is the CLI entry point: it parses args[1] as a command
// ("create", "start", or "help") and runs it. bin is the program name to
// use in usage/help output. It returns nil after printing a usage menu
// for a no-op help invocation, and a non-nil error - often a usageError,
// checkable via errors.Is against the Err*Usage sentinels - for anything
// that failed.
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
			CSMSURL:           opts.csmsUrl,
			SerialNumber:      v16.CiString25Type(opts.serialnumber),
			Vendor:            v16.CiString20Type(opts.vendor),
			Model:             v16.CiString20Type(opts.model),
			IMSI:              v16.CiString20Type(opts.imsi),
			ICCID:             v16.CiString20Type(opts.iccid),
			FirmwareVersion:   v16.CiString50Type(opts.firmwareVersion),
			MeterSerialNumber: v16.CiString25Type(opts.meterSerialnumber),
			MeterType:         v16.CiString25Type(opts.meterType),
			HeartbeatInterval: int32(opts.heartbeatInterval),
			Connectors:        []Connector{},
		}

		if err := cp.Validate(); err != nil {
			return newUsageError(ErrCreateUsage, err)
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

		fmt.Printf(`
Chargepoint state-file saved to %s
`, path)
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
		cp, err := Load(path)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrLoadFile, err)
		}

		if err := cp.Validate(); err != nil {
			return newUsageError(ErrStartUsage, err)
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
configuration.

Example: %s create ./my-chargepoint.json -id my-chargepoint

After creation, it's recommended to review and modify the state-file as needed.

Options:
  -id <string>                 (Required) Chargepoint identity
  -csms_url <url>              (Required) Websocket URL to connect to
  -sn <CiString25>             Serialnumber
  -vendor <CiString20>         Vendor name (default: "govcs")
  -model <CiString20>          Model name (default: "vcs")
  -iccid <CiString20>          Integrated Circuit Card Identifier
  -imsi <CiString20>           International Mobile Subscriber Identity
  -firmware <CiString50>       Firmware version
  -meter_sn <CiString25>       Meter serialnumber
  -meter_type <CiString25>     Meter type
  -heartbeat <seconds>         Heartbeat interval (default: 60)
  -connectors <int>            Number of connectors to scaffold (default: 2)

Run '%s --help' to see all available commands.
`, bin, bin, bin)
}

// ParseCreateFlags parses the "create" command's flags from args using
// fs, returning the populated createFlags. It only reports a parsing
// error (e.g. an unknown flag) - it doesn't validate the resulting
// values, which is the caller's job via ChargePoint.Validate.
func ParseCreateFlags(fs *flag.FlagSet, args []string) (*createFlags, error) {

	opts := createFlags{}
	fs.StringVar(&opts.id, "id", "", "chargepoint identity")
	fs.StringVar(&opts.csmsUrl, "csms_url", "", "websocket url to connect to")
	fs.StringVar(&opts.serialnumber, "sn", "", "chargepoint serialnumber")
	fs.StringVar(&opts.vendor, "vendor", "govcs", "chargepoint vendor")
	fs.StringVar(&opts.model, "model", "vcs", "chargepoint model")
	fs.StringVar(&opts.iccid, "iccid", "", "chargepoint ICCID")
	fs.StringVar(&opts.imsi, "imsi", "", "chargepoint IMSI")
	fs.StringVar(&opts.firmwareVersion, "firmware", "", "chargepoint firmware version")
	fs.StringVar(&opts.meterSerialnumber, "meter_sn", "", "chargepoint meter serialnumber")
	fs.StringVar(&opts.meterType, "meter_type", "", "chargepoint meter type")
	fs.IntVar(&opts.heartbeatInterval, "heartbeat", 60, "heartbeat interval (in seconds)")
	fs.IntVar(&opts.connectors, "connectors", 2, "number of connectors")

	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parsing error: %w", err)
	}

	return &opts, nil
}
