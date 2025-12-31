package models

// Payload represents a batch of file metadata to send to the server
type Payload struct {
	ScanID       string          `json:"scan_id"`
	AgentID      string          `json:"agent_id"`
	DatasetName  string          `json:"dataset_name"` // Required dataset name for grouping scans
	BatchNumber  int             `json:"batch_number"`
	TotalBatches *int            `json:"total_batches"` // null until final batch
	FilesystemID string          `json:"filesystem_id"`
	ScanPath     string          `json:"scan_path"`
	Files        []*FileMetadata `json:"files"`
	Stats        ScanStats       `json:"stats"`
}
