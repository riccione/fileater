package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	// Flag parsing
	dryRun := flag.Bool("dryrun", false, "Simulate the operation without moving files")
	configPath := flag.String("config", "config.json", "Path to JSON configuration file")
	recursive := flag.Bool("r", false, "Process subdirs recursively")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <path>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Organizes files recursively into categorized folders.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Validate Arguments
	if flag.NArg() != 1 {
		// fmt.Fprintf(os.Stderr, "Error: exactly one path argument is required\n")
		flag.Usage()
		os.Exit(1)
	}

	if *recursive {
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

	// Initialize Organizer
	rootPath := flag.Arg(0)
	organizer := NewOrganizer(rootPath, *dryRun, *recursive)

	// Check if the config file exists (either the default "config.json" or user provided)
	if _, err := os.Stat(*configPath); err == nil {
		if err := organizer.LoadConfig(*configPath); err != nil {
			log.Fatalf("Error loading config: %v", err)
		}
	} else {
		// Only log if the user explicitly provided a path that doesn't exist
		if flag.Lookup("config").Value.String() != "config.json" {
			log.Fatalf("Config file not found: %s", *configPath)
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
}
