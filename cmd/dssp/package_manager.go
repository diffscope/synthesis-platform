/**************************************************************************
 * DiffScope Synthesis Platform                                           *
 * Copyright (C) 2026 Team OpenVPI                                        *
 *                                                                        *
 * This program is free software: you can redistribute it and/or modify   *
 * it under the terms of the GNU General Public License as published by   *
 * the Free Software Foundation, either version 3 of the License, or      *
 * (at your option) any later version.                                    *
 *                                                                        *
 * This program is distributed in the hope that it will be useful,        *
 * but WITHOUT ANY WARRANTY; without even the implied warranty of         *
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the          *
 * GNU General Public License for more details.                           *
 *                                                                        *
 * You should have received a copy of the GNU General Public License      *
 * along with this program.  If not, see <https://www.gnu.org/licenses/>. *
 **************************************************************************/

package main

import (
	"diffscope-synthesis-platform/lib/package_manager"
	"diffscope-synthesis-platform/lib/package_manager/command"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newPMCmd() (*cobra.Command, error) {
	pmCmd := &cobra.Command{
		Use:   "pm",
		Short: "DSSP package manager",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := package_manager.InitializePackageManager()
			return err
		},
	}

	flags := pmCmd.PersistentFlags()
	flags.Bool("json", false, "Output as JSON or NDJSON for machine readability")
	flags.Bool("no-cache", false, "Disable cache and always fetch from network")
	flags.Bool("no-tty", false, "Assume stdout is not a TTY")
	if err := viper.BindPFlag("package_manager.json_output", flags.Lookup("json")); err != nil {
		return nil, err
	}
	if err := viper.BindPFlag("package_manager.no_cache", flags.Lookup("no-cache")); err != nil {
		return nil, err
	}
	if err := viper.BindPFlag("package_manager.no_tty", flags.Lookup("no-tty")); err != nil {
		return nil, err
	}

	installCmd := &cobra.Command{
		Use:   "install <package_path>...",
		Short: "Install local packages",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}
			registries, err := cmd.Flags().GetStringSlice("registry")
			if err != nil {
				return err
			}
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}

			printPMCommandOutput(cmd, args, []string{
				fmt.Sprintf("dry-run: %t", dryRun),
				fmt.Sprintf("registry: %v", registries),
				fmt.Sprintf("force: %t", force),
			})
			// TODO: implement pm install
			return nil
		},
	}
	installCmd.Flags().Bool("dry-run", false, "Preview install without changes")
	installCmd.Flags().BoolP("local-only", "l", false, "Only install from local files without registry lookup")
	installCmd.Flags().StringSliceP("registry", "r", nil, "Registry IDs")
	installCmd.Flags().BoolP("force", "f", false, "Force install")

	fetchCmd := &cobra.Command{
		Use:   "fetch <package_id@version>...",
		Short: "Fetch packages from registry",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}
			registries, err := cmd.Flags().GetStringSlice("registry")
			if err != nil {
				return err
			}
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}

			printPMCommandOutput(cmd, args, []string{
				fmt.Sprintf("dry-run: %t", dryRun),
				fmt.Sprintf("registry: %v", registries),
				fmt.Sprintf("force: %t", force),
			})
			// TODO: implement pm fetch
			return nil
		},
	}
	fetchCmd.Flags().Bool("dry-run", false, "Preview fetch without changes")
	fetchCmd.Flags().StringSliceP("registry", "r", nil, "Registry IDs")
	fetchCmd.Flags().BoolP("force", "f", false, "Force fetch")

	downloadCmd := &cobra.Command{
		Use:   "download <package_id@version>...",
		Short: "Download packages from registry",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}
			registries, err := cmd.Flags().GetStringSlice("registry")
			if err != nil {
				return err
			}

			printPMCommandOutput(cmd, args, []string{
				fmt.Sprintf("dry-run: %t", dryRun),
				fmt.Sprintf("registry: %v", registries),
			})
			// TODO: implement pm download
			return nil
		},
	}
	downloadCmd.Flags().Bool("dry-run", false, "Preview download without changes")
	downloadCmd.Flags().StringSliceP("registry", "r", nil, "Registry IDs")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update registry cache",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			registries, err := cmd.Flags().GetStringSlice("registry")
			if err != nil {
				return err
			}
			return command.Update(registries)
		},
	}
	updateCmd.Flags().StringSliceP("registry", "r", nil, "Registry IDs")

	rmCmd := &cobra.Command{
		Use:   "rm <package_id@version>...",
		Short: "Remove packages",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}

			printPMCommandOutput(cmd, args, []string{
				fmt.Sprintf("dry-run: %t", dryRun),
				fmt.Sprintf("force: %t", force),
			})
			// TODO: implement pm rm
			return nil
		},
	}
	rmCmd.Flags().Bool("dry-run", false, "Preview remove without changes")
	rmCmd.Flags().BoolP("force", "f", false, "Force remove")

	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage registries",
	}

	registrySetCmd := &cobra.Command{
		Use:   "set <registry_id> <registry_url>...",
		Short: "Set registry entries",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("requires at least one <registry_id> <registry_url> pair")
			}
			if len(args)%2 != 0 {
				return fmt.Errorf("registry set arguments must be pairs of <registry_id> <registry_url>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			entries := make([]command.RegistrySetEntry, 0, len(args)/2)
			for i := 0; i < len(args); i += 2 {
				entries = append(entries, command.RegistrySetEntry{
					ID:  args[i],
					URL: args[i+1],
				})
			}

			if err := command.SetRegistry(entries); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Note: Run `%s` to update local registry cache.\n", updateCmd.CommandPath())
			return nil
		},
	}

	registryGetCmd := &cobra.Command{
		Use:   "get [registry_id]...",
		Short: "Get registry URLs (all when omitted)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.GetRegistry(args)
		},
	}

	registryRmCmd := &cobra.Command{
		Use:   "rm <registry_id>...",
		Short: "Remove registries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := command.RmRegistry(args); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Note: Run `%s` to update local registry cache.\n", updateCmd.CommandPath())
			return nil
		},
	}

	registryCmd.AddCommand(
		registrySetCmd,
		registryGetCmd,
		registryRmCmd,
	)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all installed packages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			printPMCommandOutput(cmd, args, nil)
			// TODO: implement pm list
			return nil
		},
	}

	searchCmd := &cobra.Command{
		Use:   "search <keyword>",
		Short: "Search packages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registryID, err := cmd.Flags().GetString("registry")
			if err != nil {
				return err
			}

			printPMCommandOutput(cmd, args, []string{
				fmt.Sprintf("registry: %s", registryID),
			})
			// TODO: implement pm search
			return nil
		},
	}
	searchCmd.Flags().StringP("registry", "r", "", "Registry ID")
	searchCmd.Flags().BoolP("local-only", "l", false, "Only search local files without registry lookup")

	infoCmd := &cobra.Command{
		Use:   "info <package_id@version>",
		Short: "Get package information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registryID, err := cmd.Flags().GetString("registry")
			if err != nil {
				return err
			}

			printPMCommandOutput(cmd, args, []string{
				fmt.Sprintf("registry: %s", registryID),
			})
			// TODO: implement pm info
			return nil
		},
	}
	infoCmd.Flags().StringP("registry", "r", "", "Registry ID")
	infoCmd.Flags().BoolP("local-only", "l", false, "Only get information from local files without registry lookup")

	packCmd := &cobra.Command{
		Use:   "pack <output_package_path> <input_dir>",
		Short: "Pack package",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}

			printPMCommandOutput(cmd, args, []string{
				fmt.Sprintf("dry-run: %t", dryRun),
			})
			// TODO: implement pm pack
			return nil
		},
	}
	packCmd.Flags().Bool("dry-run", false, "Preview pack without changes")

	exportCmd := &cobra.Command{
		Use:   "export <output_package_path> <package_id@version>",
		Short: "Export installed package to a file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}

			printPMCommandOutput(cmd, args, []string{
				fmt.Sprintf("dry-run: %t", dryRun),
			})
			// TODO: implement pm export
			return nil
		},
	}
	exportCmd.Flags().Bool("dry-run", false, "Preview export without changes")

	pmCmd.AddCommand(
		installCmd,
		fetchCmd,
		downloadCmd,
		updateCmd,
		rmCmd,
		registryCmd,
		listCmd,
		searchCmd,
		infoCmd,
		packCmd,
		exportCmd,
	)

	return pmCmd, nil
}

func printPMCommandOutput(cmd *cobra.Command, args []string, details []string) {
	fmt.Fprintf(cmd.OutOrStdout(), "command: %s\n", cmd.CommandPath())
	fmt.Fprintf(cmd.OutOrStdout(), "args: %v\n", args)
	for _, detail := range details {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", detail)
	}
}
