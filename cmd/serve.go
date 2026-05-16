/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"log"
	"net/http"
	"openlambda/internal/function"
	"openlambda/internal/server/ingest"
	"openlambda/internal/server/processing"

	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the http server",
	Long:  `Start the http server that will enqueue the requests and serve the available using another endpoint`,
	Run: func(cmd *cobra.Command, args []string) {
		pendingQueue := function.NewPendingQueue()
		processingMap := function.NewProcessingMap()

		go func() {
			log.Println("Starting ingest server on :8080")
			if err := ingest.Init(pendingQueue); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("ingest listen: %s\n", err)
			}
		}()

		log.Println("Starting process server on :8081")
		if err := processing.Init(pendingQueue, processingMap); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("process listen: %s\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
