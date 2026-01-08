package reporter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brunoxghx/yemoune-agent/pkg/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// MaxBatchSizeBytes is the maximum uncompressed batch size (1GB)
	MaxBatchSizeBytes = 1024 * 1024 * 1024
)

// BatchManager manages batching of file metadata
type BatchManager struct {
	scanID       string
	agentID      string
	datasetName  string
	filesystemID string
	scanPath     string
	batchSizeMB  int
	reporter     *HTTPReporter
	logger       *zap.Logger

	currentBatch   []*models.FileMetadata
	currentSize    int64
	batchNumber    int
	totalFiles     int64
	totalBatches   int
}

// NewBatchManager creates a new batch manager
func NewBatchManager(agentID, datasetName, filesystemID, scanPath string, batchSizeMB int, reporter *HTTPReporter, logger *zap.Logger) *BatchManager {
	return &BatchManager{
		scanID:       uuid.New().String(),
		agentID:      agentID,
		datasetName:  datasetName,
		filesystemID: filesystemID,
		scanPath:     scanPath,
		batchSizeMB:  batchSizeMB,
		reporter:     reporter,
		logger:       logger,
		currentBatch: make([]*models.FileMetadata, 0, 10000),
		batchNumber:  1,
	}
}

// AddFile adds a file to the current batch
func (bm *BatchManager) AddFile(ctx context.Context, file *models.FileMetadata) error {
	// Estimate size of this file's JSON representation
	fileJSON, err := json.Marshal(file)
	if err != nil {
		return fmt.Errorf("failed to marshal file metadata: %w", err)
	}
	fileSize := int64(len(fileJSON))

	// Check if adding this file would exceed batch size
	// Reserve 2MB for JSON payload overhead (wrapper structure, formatting, etc.)
	maxBatchSize := int64(bm.batchSizeMB)*1024*1024 - 2*1024*1024
	if bm.currentSize+fileSize > maxBatchSize && len(bm.currentBatch) > 0 {
		// Send current batch
		if err := bm.sendBatch(ctx, false); err != nil {
			return err
		}
	}

	// Add file to current batch
	bm.currentBatch = append(bm.currentBatch, file)
	bm.currentSize += fileSize
	bm.totalFiles++

	return nil
}

// Flush sends any remaining files in the current batch
func (bm *BatchManager) Flush(ctx context.Context, stats models.ScanStats) error {
	if len(bm.currentBatch) > 0 {
		return bm.sendBatch(ctx, true)
	}
	return nil
}

// sendBatch sends the current batch to the server
func (bm *BatchManager) sendBatch(ctx context.Context, isFinal bool) error {
	if len(bm.currentBatch) == 0 {
		return nil
	}

	// Create payload
	payload := &models.Payload{
		ScanID:       bm.scanID,
		AgentID:      bm.agentID,
		DatasetName:  bm.datasetName,
		BatchNumber:  bm.batchNumber,
		TotalBatches: nil, // Set on final batch
		FilesystemID: bm.filesystemID,
		ScanPath:     bm.scanPath,
		Files:        bm.currentBatch,
		Stats: models.ScanStats{
			FilesScanned:       int64(len(bm.currentBatch)),
			DirectoriesScanned: 0, // Updated on final batch
			TotalSize:          0, // Updated on final batch
			Errors:             0, // Updated on final batch
		},
	}

	// Set total batches on final batch
	if isFinal {
		bm.totalBatches = bm.batchNumber
		payload.TotalBatches = &bm.totalBatches
	}

	bm.logger.Info("Sending batch",
		zap.String("scan_id", bm.scanID),
		zap.Int("batch_number", bm.batchNumber),
		zap.Int("files_in_batch", len(bm.currentBatch)),
		zap.Int64("batch_size_bytes", bm.currentSize),
		zap.Bool("is_final", isFinal))

	// Send to reporter
	if err := bm.reporter.SendPayload(ctx, payload); err != nil {
		return fmt.Errorf("failed to send batch %d: %w", bm.batchNumber, err)
	}

	// Reset current batch
	bm.currentBatch = make([]*models.FileMetadata, 0, 10000)
	bm.currentSize = 0
	bm.batchNumber++

	return nil
}

// GetScanID returns the current scan ID
func (bm *BatchManager) GetScanID() string {
	return bm.scanID
}
