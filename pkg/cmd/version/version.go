package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"gitverse.ru/veryviolet/gv/internal/build"
	"gitverse.ru/veryviolet/gv/internal/cmdutil"
)

// NewCmdVersion returns the `gv version` command.
func NewCmdVersion(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of gv",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(f.IOStreams.Out, Format())
			return nil
		},
	}
}

// Format renders the version banner shared by `gv version` and `gv --version`.
func Format() string {
	s := fmt.Sprintf("gv version %s", build.Version)
	if build.Date != "" {
		s += fmt.Sprintf(" (%s)", build.Date)
	}
	return s + "\n"
}
