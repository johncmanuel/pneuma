package models

// ClampPagination constrains offset and limit to fixed ranges for pagination.
// Defaults limit to 50 if zero or negative, and caps it at 200.
// Ensures offset is non-negative.
func ClampPagination(offset, limit int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return offset, limit
}
