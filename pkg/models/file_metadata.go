package models

import "time"

// FileMetadata represents metadata for a single file
type FileMetadata struct {
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	Inode         uint64    `json:"inode"`
	LinkCount     uint32    `json:"link_count"`
	FilesystemID  string    `json:"filesystem_id"`
	OwnerUID      uint32    `json:"owner_uid"`
	OwnerGID      uint32    `json:"owner_gid"`
	Permissions   uint32    `json:"permissions"`
	ModifiedTime  time.Time `json:"modified_time"`
	AccessedTime  time.Time `json:"accessed_time"`
	ChangedTime   time.Time `json:"changed_time"`
	FileType      string    `json:"file_type"` // regular, directory, symlink
	Extension     string    `json:"extension"`
}

// ScanStats represents statistics for a scan batch
type ScanStats struct {
	FilesScanned       int64 `json:"files_scanned"`
	DirectoriesScanned int64 `json:"directories_scanned"`
	TotalSize          int64 `json:"total_size"`
	Errors             int64 `json:"errors"`
}
