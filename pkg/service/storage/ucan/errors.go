package ucan

import (
	"fmt"

	"github.com/storacha/go-ucanto/ucan"
)

type UnsupportedCapabilityError[C any] struct {
	capability ucan.Capability[C]
}

func (ue UnsupportedCapabilityError[C]) Name() string {
	return "UnsupportedCapability"
}

func (ue UnsupportedCapabilityError[C]) Capability() ucan.Capability[C] {
	return ue.capability
}

func (ue UnsupportedCapabilityError[C]) Error() string {
	return fmt.Sprintf(`%s does not have a "%s" capability provider`, ue.capability.With(), ue.capability.Can())
}

func NewUnsupportedCapabilityError[C any](capability ucan.Capability[C]) UnsupportedCapabilityError[C] {
	return UnsupportedCapabilityError[C]{capability}
}

type BlobSizeLimitExceededError struct {
	size uint64
	max  uint64
}

func (be BlobSizeLimitExceededError) Name() string {
	return "BlobSizeOutsideOfSupportedRange"
}

func (be BlobSizeLimitExceededError) Error() string {
	return fmt.Sprintf("Blob of %d bytes, exceeds size limit of %d bytes", be.size, be.max)
}

func NewBlobSizeLimitExceededError(size uint64, max uint64) BlobSizeLimitExceededError {
	return BlobSizeLimitExceededError{size, max}
}

type AllocatedMemoryNotWrittenError struct{}

func (ae AllocatedMemoryNotWrittenError) Name() string {
	return "AllocatedMemoryHadNotBeenWrittenTo"
}

func (ae AllocatedMemoryNotWrittenError) Error() string {
	return "Blob not found"
}

func NewAllocatedMemoryNotWrittenError() AllocatedMemoryNotWrittenError {
	return AllocatedMemoryNotWrittenError{}
}
