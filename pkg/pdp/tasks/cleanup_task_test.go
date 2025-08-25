package tasks

import (
	"context"
	"io"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"github.com/storacha/piri/pkg/pdp/scheduler"
	"github.com/storacha/piri/pkg/pdp/service/models"
	"github.com/storacha/piri/pkg/store/blobstore"
)

// Mock implementations
type MockStash struct {
	mock.Mock
}

func (m *MockStash) StashCreate(ctx context.Context, maxSize int64, writeFunc func(f *os.File) error) (uuid.UUID, error) {
	args := m.Called(ctx, maxSize, writeFunc)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockStash) StashRemove(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStash) StashURL(id uuid.UUID) (url.URL, error) {
	args := m.Called(id)
	return args.Get(0).(url.URL), args.Error(1)
}

type MockBlobstore struct {
	mock.Mock
}

func (m *MockBlobstore) Put(ctx context.Context, digest multihash.Multihash, size uint64, body io.Reader) error {
	args := m.Called(ctx, digest, size, body)
	return args.Error(0)
}

func (m *MockBlobstore) Get(ctx context.Context, digest multihash.Multihash, opts ...blobstore.GetOption) (blobstore.Object, error) {
	args := m.Called(ctx, digest, opts)
	return args.Get(0).(blobstore.Object), args.Error(1)
}

func TestCleanupTask_ExtractStashIDFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name:    "valid stash URL",
			url:     "file:///path/to/stash/123e4567-e89b-12d3-a456-426614174000.tmp",
			want:    "123e4567-e89b-12d3-a456-426614174000",
			wantErr: false,
		},
		{
			name:    "invalid URL scheme",
			url:     "http://path/to/stash/uuid.tmp",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid filename format",
			url:     "file:///path/to/stash/invalid.txt",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractStashIDFromURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCleanupTask_CleanupStash(t *testing.T) {
	mockStash := &MockStash{}
	mockBlobstore := &MockBlobstore{}

	cleanupTask := &CleanupTask{
		db: nil, // Will be mocked
		bs: mockBlobstore,
		ss: mockStash,
	}

	ctx := context.Background()
	taskID := scheduler.TaskID(1)
	piece := models.ParkedPiece{
		ID: 1,
	}

	// Mock database calls
	mockDB := &MockDB{}
	mockDB.On("WithContext", ctx).Return(mockDB)
	mockDB.On("Where", "piece_id = ?", int64(1)).Return(mockDB)
	mockDB.On("First", mock.AnythingOfType("*models.ParkedPieceRef")).Return(nil)

	// Mock stash removal
	stashID, _ := uuid.Parse("123e4567-e89b-12d3-a456-426614174000")
	mockStash.On("StashRemove", ctx, stashID).Return(nil)

	// Mock cleanup task completion
	mockDB.On("Model", mock.AnythingOfType("*models.ParkedPiece")).Return(mockDB)
	mockDB.On("Where", "id = ?", int64(1)).Return(mockDB)
	mockDB.On("Update", "cleanup_task_id", nil).Return(mockDB)
	mockDB.On("Error").Return(nil)

	done, err := cleanupTask.cleanupStash(ctx, taskID, piece)

	assert.NoError(t, err)
	assert.True(t, done)
	mockStash.AssertExpectations(t)
}

func TestCleanupTask_CleanupBlob(t *testing.T) {
	mockStash := &MockStash{}
	mockBlobstore := &MockBlobstore{}

	cleanupTask := &CleanupTask{
		db: nil, // Will be mocked
		bs: mockBlobstore,
		ss: mockStash,
	}

	ctx := context.Background()
	taskID := scheduler.TaskID(1)
	pieceRef := models.PDPPieceRef{
		ID:       1,
		PieceCID: "baga6ea4seqa",
	}

	// Mock database calls
	mockDB := &MockDB{}
	mockDB.On("WithContext", ctx).Return(mockDB)
	mockDB.On("Delete", &pieceRef).Return(mockDB)
	mockDB.On("Error").Return(nil)

	done, err := cleanupTask.cleanupBlob(ctx, taskID, pieceRef)

	assert.NoError(t, err)
	assert.True(t, done)
}

// MockDB is a mock implementation of *gorm.DB
type MockDB struct {
	mock.Mock
}

func (m *MockDB) WithContext(ctx context.Context) *gorm.DB {
	args := m.Called(ctx)
	return args.Get(0).(*gorm.DB)
}

func (m *MockDB) Where(query interface{}, args ...interface{}) *gorm.DB {
	mockArgs := m.Called(query, args)
	return mockArgs.Get(0).(*gorm.DB)
}

func (m *MockDB) First(dest interface{}, conds ...interface{}) *gorm.DB {
	mockArgs := m.Called(dest, conds)
	return mockArgs.Get(0).(*gorm.DB)
}

func (m *MockDB) Model(value interface{}) *gorm.DB {
	mockArgs := m.Called(value)
	return mockArgs.Get(0).(*gorm.DB)
}

func (m *MockDB) Update(column string, value interface{}) *gorm.DB {
	mockArgs := m.Called(column, value)
	return mockArgs.Get(0).(*gorm.DB)
}

func (m *MockDB) Delete(value interface{}, conds ...interface{}) *gorm.DB {
	mockArgs := m.Called(value, conds)
	return mockArgs.Get(0).(*gorm.DB)
}

func (m *MockDB) Error() error {
	args := m.Called()
	return args.Error(0)
}
