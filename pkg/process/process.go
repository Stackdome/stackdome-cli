package process

import (
	"os"

	"github.com/spf13/pflag"
)

func GetCurrentProcessFlag(flag string) *string {
	flagRef := pflag.String(flag, "", "target")
	pflag.CommandLine.Parse(os.Args[1:])
	if flagRef != nil && len(*flagRef) != 0 {
		return flagRef
	}
	return nil
}
