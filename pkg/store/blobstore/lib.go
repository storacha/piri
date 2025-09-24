package blobstore

// rangeSatisfiable determines if the provided start/end byte range is
// valid given the total size of the blob.
func rangeSatisfiable(start uint64, end *uint64, size uint64) bool {
	if start >= size {
		return false
	}
	if end != nil {
		if start > *end {
			return false
		}
		if *end >= size {
			return false
		}
	}
	return true
}
