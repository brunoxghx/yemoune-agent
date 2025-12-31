package scanner

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// GetFilesystemID returns the filesystem ID for a given path
// This uses statfs to get the filesystem UUID which remains constant
// across mount points and multipath configurations
func GetFilesystemID(path string) (string, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return "", fmt.Errorf("statfs failed for %s: %w", path, err)
	}

	// Fsid is unique per filesystem
	// Format as hex string - use Val array (works across Linux architectures)
	fsidBytes := (*[2]int32)(unsafe.Pointer(&stat.Fsid))
	fsid := fmt.Sprintf("%x-%x", fsidBytes[0], fsidBytes[1])
	return fsid, nil
}

// GetFileExtension extracts the file extension from a path
func GetFileExtension(path string) string {
	ext := filepath.Ext(path)
	if ext != "" && len(ext) > 1 {
		return ext[1:] // Remove leading dot
	}
	return ""
}

// ShouldExclude checks if a path should be excluded based on patterns
func ShouldExclude(path string, excludePatterns []string) bool {
	for _, pattern := range excludePatterns {
		// Simple glob matching
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}

		// Check if path contains pattern (for directory exclusions like /proc/*)
		if strings.Contains(path, strings.TrimSuffix(pattern, "/*")) {
			return true
		}
	}
	return false
}

// GetFileType determines the file type from mode
func GetFileType(mode uint32) string {
	switch mode & unix.S_IFMT {
	case unix.S_IFREG:
		return "regular"
	case unix.S_IFDIR:
		return "directory"
	case unix.S_IFLNK:
		return "symlink"
	case unix.S_IFBLK:
		return "block"
	case unix.S_IFCHR:
		return "char"
	case unix.S_IFIFO:
		return "fifo"
	case unix.S_IFSOCK:
		return "socket"
	default:
		return "unknown"
	}
}
