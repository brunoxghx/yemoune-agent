package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/brunoxghx/yemoune-agent/internal/config"
	"github.com/brunoxghx/yemoune-agent/internal/reporter"
	"github.com/brunoxghx/yemoune-agent/internal/scanner"
	"github.com/brunoxghx/yemoune-agent/pkg/models"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	workers     int
	dryRun      bool
	datasetName string
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a directory and report file metadata",
	Long: `Scan recursively scans a directory and collects file metadata.
The metadata is batched and sent to the configured backend server.

The --dataset flag is required and specifies the dataset name for grouping
multiple scans together (e.g., different snapshots of the same filesystem).`,
	Args: cobra.ExactArgs(1),
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().IntVar(&workers, "workers", 0, "number of parallel workers (0 = use config)")
	scanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "scan without sending to server")
	scanCmd.Flags().StringVar(&datasetName, "dataset", "", "dataset name for grouping scans (required)")
	scanCmd.MarkFlagRequired("dataset")
}

func runScan(cmd *cobra.Command, args []string) error {
	scanPath := args[0]

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Override workers if specified
	if workers > 0 {
		cfg.Scanner.Workers = workers
	}

	// Setup logger
	logger, err := setupLogger(cfg.Logging.Level, cfg.Logging.Format, verbose)
	if err != nil {
		return fmt.Errorf("failed to setup logger: %w", err)
	}
	defer logger.Sync()

	logger.Info("Starting scan",
		zap.String("path", scanPath),
		zap.String("dataset", datasetName),
		zap.String("agent_id", cfg.Agent.ID),
		zap.Int("workers", cfg.Scanner.Workers),
		zap.Bool("dry_run", dryRun))

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Received interrupt signal, stopping scan...")
		cancel()
	}()

	// Get filesystem ID
	fsid, err := scanner.GetFilesystemID(scanPath)
	if err != nil {
		return fmt.Errorf("failed to get filesystem ID: %w", err)
	}

	// Create HTTP reporter
	var httpReporter *reporter.HTTPReporter
	if !dryRun {
		httpReporter, err = reporter.NewHTTPReporter(
			cfg.Agent.ServerURL,
			cfg.Agent.APIToken,
			cfg.Reporter.TimeoutSeconds,
			cfg.Reporter.CompressionLevel,
			cfg.Reporter.MaxRetries,
			cfg.Reporter.RetryDelaySeconds,
			logger,
		)
		if err != nil {
			return fmt.Errorf("failed to create HTTP reporter: %w", err)
		}
		defer httpReporter.Close()
	}

	// Create batch manager
	var batchMgr *reporter.BatchManager
	if !dryRun {
		batchMgr = reporter.NewBatchManager(
			cfg.Agent.ID,
			datasetName,
			fsid,
			scanPath,
			cfg.Scanner.BatchSizeMB,
			httpReporter,
			logger,
		)
	}

	// Create scanner
	fileScanner := scanner.NewScanner(
		cfg.Scanner.Workers,
		cfg.Scanner.MaxDepth,
		cfg.Scanner.ExcludePaths,
		cfg.Scanner.MinFileSize,
		cfg.Scanner.MaxFileSize,
		logger,
	)

	// Create channel for file metadata
	filesCh := make(chan *models.FileMetadata, 1000)

	// Start file collector goroutine
	errCh := make(chan error, 1)
	go func() {
		for file := range filesCh {
			if dryRun {
				logger.Debug("File found",
					zap.String("path", file.Path),
					zap.Int64("size", file.Size))
			} else {
				if err := batchMgr.AddFile(ctx, file); err != nil {
					errCh <- fmt.Errorf("failed to add file to batch: %w", err)
					return
				}
			}
		}
		errCh <- nil
	}()

	// Start scan
	if err := fileScanner.Scan(ctx, scanPath, filesCh); err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Wait for file collector to finish
	if err := <-errCh; err != nil {
		return err
	}

	// Flush remaining batch
	if !dryRun {
		stats := fileScanner.GetStats()
		if err := batchMgr.Flush(ctx, stats); err != nil {
			return fmt.Errorf("failed to flush final batch: %w", err)
		}

		logger.Info("Scan completed successfully",
			zap.String("scan_id", batchMgr.GetScanID()),
			zap.Int64("files", stats.FilesScanned),
			zap.Int64("directories", stats.DirectoriesScanned),
			zap.Int64("total_size", stats.TotalSize),
			zap.Int64("errors", stats.Errors))
	} else {
		stats := fileScanner.GetStats()
		logger.Info("Dry-run scan completed",
			zap.Int64("files", stats.FilesScanned),
			zap.Int64("directories", stats.DirectoriesScanned),
			zap.Int64("total_size", stats.TotalSize),
			zap.Int64("errors", stats.Errors))
	}

	return nil
}
