package cli

import (
	"context"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/AngheloAlva/timer/internal/updater"
	"github.com/AngheloAlva/timer/internal/version"
)

func newUpdateCmd() *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"upgrade"},
		Short:   "Check for and install the latest timer release",
		Long: `Detect how timer was installed and update it accordingly.

For Homebrew installs, runs 'brew upgrade --cask timer'. For standalone
binaries (Windows zip, manual download, etc.), downloads the matching
asset from GitHub and replaces the running binary in place.

Use --check to print the latest available version without installing.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.Context(), checkOnly)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only check for a newer release; do not install")
	return cmd
}

func runUpdate(ctx context.Context, checkOnly bool) error {
	fmt.Printf("Current version: %s\n", version.Version)

	rel, err := updater.FetchLatest(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Latest release:  %s\n", rel.TagName)

	if version.Version == "dev" {
		fmt.Println("\nRunning a dev build (locally compiled). Skipping auto-update.")
		fmt.Printf("See: %s\n", rel.HTMLURL)
		return nil
	}
	if !updater.IsNewer(rel.TagName, version.Version) {
		fmt.Println("\nAlready up to date.")
		return nil
	}

	channel, exePath, err := updater.DetectChannel()
	if err != nil {
		return err
	}
	fmt.Printf("Detected channel: %s (%s)\n", channel, exePath)

	if checkOnly {
		fmt.Println("\nA newer version is available. Run 'timer update' to install.")
		return nil
	}

	switch channel {
	case updater.ChannelBrew:
		fmt.Println("\nRunning: brew upgrade --cask timer")
		return updater.UpgradeBrew(ctx)
	case updater.ChannelStandalone:
		asset := updater.FindAsset(rel)
		if asset == nil {
			return fmt.Errorf("no release asset found for %s/%s — download manually: %s",
				runtime.GOOS, runtime.GOARCH, rel.HTMLURL)
		}
		fmt.Printf("\nDownloading %s...\n", asset.Name)
		if err := updater.ApplyStandalone(ctx, asset); err != nil {
			return err
		}
		fmt.Println("\nUpdate applied. Restart timer to use the new version.")
		return nil
	default:
		fmt.Printf("\nUnknown installation channel. Download manually: %s\n", rel.HTMLURL)
		return nil
	}
}
