package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	Version    = "dev"
	dryRun     bool
	configPath string
	recursive  bool
	force      bool
	logPath    string
	minSize    string
	maxSize   string
	deleteDupes bool
	undo       bool
)

func main() {
	Execute()
}

var rootCmd = &cobra.Command{
	Use:     "fileater [path]",
	Short:   "Organizes files recursively into categorized folders",
	Version: Version,
	Args:    cobra.ExactArgs(1), // Enforces exactly one path argument
	Run: func(cmd *cobra.Command, args []string) {
		rootPath := args[0]

		if undo {
			if err := Undo(rootPath, dryRun); err != nil {
				log.Fatalf("Undo failed: %v", err)
			}
			log.Println("Undo completed successfully.")
			return
		}

		if recursive && !force {
			fmt.Println("WARNING: Recursive mode enabled. This will move files out of their current subdirs")
			fmt.Print("Are you sure you want to proceed? (y/N): ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println("Operation cancelled.")
				return
			}
		}

		// Setup context with Signal Handling (Ctrl+C)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Setup logger
		var logger *slog.Logger
		if logPath != "" {
			logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatalf("Error opening log file: %v", err)
			}
			defer logFile.Close()
			logger = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
				AddSource: true,
			}))
		} else {
			logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				AddSource: true,
			}))
		}

		// Initialize Organizer
		organizer, err := NewOrganizer(rootPath, dryRun, recursive, logger, minSize, maxSize, deleteDupes)
		if err != nil {
			log.Fatalf("Error initializing organizer: %v", err)
		}

		// Check if the config file exists (either the default "config.json" or user provided)
		if _, err := os.Stat(configPath); err == nil {
			if err := organizer.LoadConfig(configPath); err != nil {
				log.Fatalf("Error loading config: %v", err)
			}
		} else {
			// Only log if the user explicitly provided a path that doesn't exist
			if flag.Lookup("config").Value.String() != "config.json" {
				log.Fatalf("Config file not found: %s", configPath)
			}
			// If "config.json" is missing, use internal defaults
			organizer.UseDefaultCategories()
		}

		// Execute
		log.Printf("Starting organization of: %s", rootPath)
		if err := organizer.Run(ctx); err != nil {
			if err == context.Canceled {
				log.Println("Operation cancelled by user.")
			} else {
				log.Fatalf("Fatal error: %v", err)
			}
		}

		log.Println("Process completed successfully.")
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dryrun", "d", false, "Simulate the operation without moving files")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.json", "Path to JSON configuration file")
	rootCmd.PersistentFlags().BoolVarP(&recursive, "recursive", "r", false, "Process subdirs recursively")
	rootCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt (for scripts/automation)")
	rootCmd.PersistentFlags().StringVarP(&logPath, "log", "l", "", "Path to log file (appended if exists)")
	rootCmd.PersistentFlags().StringVar(&minSize, "min-size", "", "Minimum file size (e.g., 100KB, 10MB, 1GB)")
	rootCmd.PersistentFlags().StringVar(&maxSize, "max-size", "", "Maximum file size (e.g., 100KB, 10MB, 1GB)")
	rootCmd.PersistentFlags().BoolVarP(&deleteDupes, "delete-dupes", "", false, "Delete duplicate files instead of skipping")
	rootCmd.PersistentFlags().BoolVar(&undo, "undo", false, "Undo the last organization run and restore original directory structure")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
