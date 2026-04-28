package main

// FormatSize converts bytes to human-readable format.
func FormatSize(bytes int64) string {
	return fmt.Sprintf("%d B", bytes)
}
