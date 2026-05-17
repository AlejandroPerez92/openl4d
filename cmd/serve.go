/*
Copyright © 2026 Alejandro Pérez <alxpefa@gmail.com>
*/
package cmd

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlejandroPerez92/openl4d/internal/function"
	"github.com/AlejandroPerez92/openl4d/internal/server/ingest"
	"github.com/AlejandroPerez92/openl4d/internal/server/processing"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the http server",
	Long:  `Start the http server that will enqueue the requests and serve the available using another endpoint`,
	Run: func(cmd *cobra.Command, args []string) {
		pendingQueue := function.NewPendingQueue(4)
		processingMap := function.NewProcessingMap()
		sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		ingestServer := ingest.NewServer(pendingQueue, processingMap)
		processServer := processing.NewServer(pendingQueue, processingMap)

		type serverResult struct {
			name string
			err  error
		}
		serverResults := make(chan serverResult, 2)

		go func() {
			log.Println("Starting ingest server on :8080")
			err := ingestServer.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				serverResults <- serverResult{name: "ingest", err: nil}
				return
			}
			serverResults <- serverResult{name: "ingest", err: err}
		}()

		go func() {
			log.Println("Starting process server on :8081")
			err := processServer.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				serverResults <- serverResult{name: "process", err: nil}
				return
			}
			serverResults <- serverResult{name: "process", err: err}
		}()

		select {
		case <-sigCtx.Done():
			log.Println("Signal received: closing pending queue input and draining in-flight work")
			pendingQueue.Close()

			ingestShutdownCtx, ingestCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := ingestServer.Shutdown(ingestShutdownCtx); err != nil {
				log.Printf("ingest shutdown error: %v", err)
			}
			ingestCancel()

			drainDeadline := time.Now().Add(30 * time.Second)
			for {
				if pendingQueue.Len() == 0 && processingMap.Len() == 0 {
					break
				}
				if time.Now().After(drainDeadline) {
					log.Printf("drain deadline reached: pending=%d processing=%d", pendingQueue.Len(), processingMap.Len())
					break
				}
				time.Sleep(100 * time.Millisecond)
			}

			processShutdownCtx, processCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := processServer.Shutdown(processShutdownCtx); err != nil {
				log.Printf("process shutdown error: %v", err)
			}
			processCancel()

			for i := 0; i < 2; i++ {
				result := <-serverResults
				if result.err != nil {
					log.Printf("%s server exited with error: %v", result.name, result.err)
				}
			}
			log.Println("Shutdown complete")
		case result := <-serverResults:
			if result.err != nil {
				log.Fatalf("%s listen failed: %v", result.name, result.err)
			}
			log.Fatalf("%s server exited unexpectedly", result.name)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
