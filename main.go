package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	// Flag parsing
	dryRun := flag.Bool("dryrun", false, "Simulate the operation without moving files")
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

	// Setup context with Signal Handling (Ctrl+C)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Organizer
	rootPath := flag.Arg(0)
	organizer := NewOrganizer(rootPath, *dryRun)

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
