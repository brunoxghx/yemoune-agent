package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brunoxghx/yemoune-agent/pkg/models"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// timespecToTime converts unix.Timespec to time.Time
func timespecToTime(ts unix.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec)
}

// Scanner performs parallel file system scanning
type Scanner struct {
	workers        int
	maxDepth       int
	excludePaths   []string
	minFileSize    int64
	maxFileSize    int64
	logger         *zap.Logger
	filesystemID   string

	// Statistics
	filesScanned  int64
	dirsScanned   int64
	totalSize     int64
	errorCount    int64
}

// NewScanner creates a new Scanner instance
func NewScanner(workers int, maxDepth int, excludePaths []string, minSize, maxSize int64, logger *zap.Logger) *Scanner {
	return &Scanner{
		workers:      workers,
		maxDepth:     maxDepth,
		excludePaths: excludePaths,
		minFileSize:  minSize,
		maxFileSize:  maxSize,
		logger:       logger,
	}
}

// Scan scans a directory and sends file metadata through the channel
func (s *Scanner) Scan(ctx context.Context, rootPath string, filesCh chan<- *models.FileMetadata) error {
	// Get filesystem ID for this scan
	fsid, err := GetFilesystemID(rootPath)
	if err != nil {
		return fmt.Errorf("failed to get filesystem ID: %w", err)
	}
	s.filesystemID = fsid
	s.logger.Info("Starting scan",
		zap.String("path", rootPath),
		zap.String("filesystem_id", fsid),
		zap.Int("workers", s.workers))

	// Use a semaphore-based approach instead of a work queue
	// This allows us to process directories recursively without deadlock
	var wg sync.WaitGroup
	sem := make(chan struct{}, s.workers)

	// Start the scan with the root directory
	wg.Add(1)
	go s.scanDirectoryRecursive(ctx, rootPath, filesCh, &wg, sem, 0)

	// Wait for all scanning to complete
	wg.Wait()
	close(filesCh)

	s.logger.Info("Scan completed",
		zap.Int64("files_scanned", s.filesScanned),
		zap.Int64("directories_scanned", s.dirsScanned),
		zap.Int64("total_size", s.totalSize),
		zap.Int64("errors", s.errorCount))

	return nil
}

// scanDirectoryRecursive scans a directory recursively with semaphore-based concurrency control
func (s *Scanner) scanDirectoryRecursive(ctx context.Context, path string, filesCh chan<- *models.FileMetadata, wg *sync.WaitGroup, sem chan struct{}, depth int) {
	defer wg.Done()

	// Acquire semaphore
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Check depth limit
	if s.maxDepth > 0 && depth > s.maxDepth {
		return
	}

	// Check if path should be excluded
	if ShouldExclude(path, s.excludePaths) {
		s.logger.Debug("Excluding path", zap.String("path", path))
		return
	}

	// Scan this directory
	s.scanDirectorySingle(ctx, path, filesCh, wg, sem, depth)
}

// scanDirectorySingle scans a single directory and spawns goroutines for subdirectories
func (s *Scanner) scanDirectorySingle(ctx context.Context, path string, filesCh chan<- *models.FileMetadata, wg *sync.WaitGroup, sem chan struct{}, depth int) {
	// Open directory
	dir, err := os.Open(path)
	if err != nil {
		atomic.AddInt64(&s.errorCount, 1)
		s.logger.Warn("Failed to open directory",
			zap.String("path", path),
			zap.Error(err))
		return
	}
	defer dir.Close()

	// Read directory entries
	entries, err := dir.Readdir(-1)
	if err != nil {
		atomic.AddInt64(&s.errorCount, 1)
		s.logger.Warn("Failed to read directory",
			zap.String("path", path),
			zap.Error(err))
		return
	}

	atomic.AddInt64(&s.dirsScanned, 1)

	// Process each entry
	for _, entry := range entries {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		entryPath := filepath.Join(path, entry.Name())

		// Get file info with stat
		var stat unix.Stat_t
		if err := unix.Lstat(entryPath, &stat); err != nil {
			atomic.AddInt64(&s.errorCount, 1)
			s.logger.Debug("Failed to stat file",
				zap.String("path", entryPath),
				zap.Error(err))
			continue
		}

		// If it's a directory, spawn a new goroutine to process it
		if entry.IsDir() {
			wg.Add(1)
			go s.scanDirectoryRecursive(ctx, entryPath, filesCh, wg, sem, depth+1)
			continue
		}

		// Process file
		fileType := GetFileType(uint32(stat.Mode))

		// Skip non-regular files unless they're symlinks
		if fileType != "regular" && fileType != "symlink" {
			continue
		}

		// Apply size filters
		if s.minFileSize > 0 && stat.Size < s.minFileSize {
			continue
		}
		if s.maxFileSize > 0 && stat.Size > s.maxFileSize {
			continue
		}

		// Create file metadata
		metadata := &models.FileMetadata{
			Path:          entryPath,
			Size:          stat.Size,
			Inode:         stat.Ino,
			LinkCount:     uint32(stat.Nlink),
			FilesystemID:  s.filesystemID,
			OwnerUID:      stat.Uid,
			OwnerGID:      stat.Gid,
			Permissions:   uint32(stat.Mode & 0777),
			ModifiedTime:  timespecToTime(stat.Mtim),
			AccessedTime:  timespecToTime(stat.Atim),
			ChangedTime:   timespecToTime(stat.Ctim),
			FileType:      fileType,
			Extension:     GetFileExtension(entryPath),
		}

		// Send to files channel
		select {
		case filesCh <- metadata:
			atomic.AddInt64(&s.filesScanned, 1)
			atomic.AddInt64(&s.totalSize, stat.Size)
		case <-ctx.Done():
			return
		}
	}
}

// GetStats returns current scan statistics
func (s *Scanner) GetStats() models.ScanStats {
	return models.ScanStats{
		FilesScanned:       atomic.LoadInt64(&s.filesScanned),
		DirectoriesScanned: atomic.LoadInt64(&s.dirsScanned),
		TotalSize:          atomic.LoadInt64(&s.totalSize),
		Errors:             atomic.LoadInt64(&s.errorCount),
	}
}
