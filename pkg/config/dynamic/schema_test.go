package dynamic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDurationSchema_ParseAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		schema  DurationSchema
		input   any
		want    time.Duration
		wantErr bool
		errType any
	}{
		{
			name:   "parses string 30s",
			schema: DurationSchema{Min: time.Second, Max: time.Hour},
			input:  "30s",
			want:   30 * time.Second,
		},
		{
			name:   "parses string 5m",
			schema: DurationSchema{Min: time.Second, Max: time.Hour},
			input:  "5m",
			want:   5 * time.Minute,
		},
		{
			name:   "parses string 1h",
			schema: DurationSchema{Min: time.Second, Max: 2 * time.Hour},
			input:  "1h",
			want:   time.Hour,
		},
		{
			name:   "accepts time.Duration directly",
			schema: DurationSchema{Min: time.Second, Max: time.Hour},
			input:  30 * time.Second,
			want:   30 * time.Second,
		},
		{
			name:    "rejects invalid string",
			schema:  DurationSchema{Min: time.Second, Max: time.Hour},
			input:   "invalid",
			wantErr: true,
			errType: &ParseError{},
		},
		{
			name:    "rejects int type",
			schema:  DurationSchema{Min: time.Second, Max: time.Hour},
			input:   30,
			wantErr: true,
			errType: &TypeError{},
		},
		{
			name:    "rejects bool type",
			schema:  DurationSchema{Min: time.Second, Max: time.Hour},
			input:   true,
			wantErr: true,
			errType: &TypeError{},
		},
		{
			name:    "validates min constraint",
			schema:  DurationSchema{Min: time.Minute, Max: time.Hour},
			input:   "1s",
			wantErr: true,
			errType: &RangeError[time.Duration]{},
		},
		{
			name:    "validates max constraint",
			schema:  DurationSchema{Min: time.Second, Max: time.Minute},
			input:   "2m",
			wantErr: true,
			errType: &RangeError[time.Duration]{},
		},
		{
			name:   "accepts value at min boundary",
			schema: DurationSchema{Min: time.Minute, Max: time.Hour},
			input:  "1m",
			want:   time.Minute,
		},
		{
			name:   "accepts value at max boundary",
			schema: DurationSchema{Min: time.Second, Max: time.Hour},
			input:  "1h",
			want:   time.Hour,
		},
		{
			name:   "zero min allows any positive duration",
			schema: DurationSchema{Min: 0, Max: time.Hour},
			input:  "1ns",
			want:   time.Nanosecond,
		},
		{
			name:   "zero max allows any duration",
			schema: DurationSchema{Min: time.Second, Max: 0},
			input:  "1000h",
			want:   1000 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.schema.ParseAndValidate(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.IsType(t, tt.errType, err)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDurationSchema_TypeDescription(t *testing.T) {
	schema := DurationSchema{Min: time.Second, Max: time.Hour}
	desc := schema.TypeDescription()
	require.Contains(t, desc, "duration")
	require.Contains(t, desc, "1s")
	require.Contains(t, desc, "1h0m0s")
}

func TestIntSchema_ParseAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		schema  IntSchema
		input   any
		want    int
		wantErr bool
		errType any
	}{
		{
			name:   "parses int directly",
			schema: IntSchema{Min: 0, Max: 100},
			input:  50,
			want:   50,
		},
		{
			name:   "parses int64",
			schema: IntSchema{Min: 0, Max: 100},
			input:  int64(50),
			want:   50,
		},
		{
			name:   "parses float64 whole number",
			schema: IntSchema{Min: 0, Max: 100},
			input:  float64(50),
			want:   50,
		},
		{
			name:   "parses string",
			schema: IntSchema{Min: 0, Max: 100},
			input:  "50",
			want:   50,
		},
		{
			name:   "parses negative int",
			schema: IntSchema{Min: -100, Max: 100},
			input:  -50,
			want:   -50,
		},
		{
			name:    "rejects float64 with decimal",
			schema:  IntSchema{Min: 0, Max: 100},
			input:   50.5,
			wantErr: true,
			errType: &ParseError{},
		},
		{
			name:    "rejects invalid string",
			schema:  IntSchema{Min: 0, Max: 100},
			input:   "not a number",
			wantErr: true,
			errType: &ParseError{},
		},
		{
			name:    "rejects bool type",
			schema:  IntSchema{Min: 0, Max: 100},
			input:   true,
			wantErr: true,
			errType: &TypeError{},
		},
		{
			name:    "validates min constraint",
			schema:  IntSchema{Min: 10, Max: 100},
			input:   5,
			wantErr: true,
			errType: &RangeError[int]{},
		},
		{
			name:    "validates max constraint",
			schema:  IntSchema{Min: 0, Max: 100},
			input:   150,
			wantErr: true,
			errType: &RangeError[int]{},
		},
		{
			name:   "accepts value at min boundary",
			schema: IntSchema{Min: 10, Max: 100},
			input:  10,
			want:   10,
		},
		{
			name:   "accepts value at max boundary",
			schema: IntSchema{Min: 0, Max: 100},
			input:  100,
			want:   100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.schema.ParseAndValidate(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.IsType(t, tt.errType, err)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIntSchema_TypeDescription(t *testing.T) {
	schema := IntSchema{Min: 1, Max: 100}
	desc := schema.TypeDescription()
	require.Contains(t, desc, "integer")
	require.Contains(t, desc, "1")
	require.Contains(t, desc, "100")
}

func TestUintSchema_ParseAndValidate(t *testing.T) {
	tests := []struct {
		name    string
		schema  UintSchema
		input   any
		want    uint
		wantErr bool
		errType any
	}{
		{
			name:   "parses uint directly",
			schema: UintSchema{Min: 0, Max: 100},
			input:  uint(50),
			want:   50,
		},
		{
			name:   "parses positive int",
			schema: UintSchema{Min: 0, Max: 100},
			input:  50,
			want:   50,
		},
		{
			name:   "parses positive int64",
			schema: UintSchema{Min: 0, Max: 100},
			input:  int64(50),
			want:   50,
		},
		{
			name:   "parses positive float64 whole number",
			schema: UintSchema{Min: 0, Max: 100},
			input:  float64(50),
			want:   50,
		},
		{
			name:   "parses string",
			schema: UintSchema{Min: 0, Max: 100},
			input:  "50",
			want:   50,
		},
		{
			name:    "rejects negative int",
			schema:  UintSchema{Min: 0, Max: 100},
			input:   -5,
			wantErr: true,
			errType: &ParseError{},
		},
		{
			name:    "rejects negative int64",
			schema:  UintSchema{Min: 0, Max: 100},
			input:   int64(-5),
			wantErr: true,
			errType: &ParseError{},
		},
		{
			name:    "rejects negative float64",
			schema:  UintSchema{Min: 0, Max: 100},
			input:   float64(-5),
			wantErr: true,
			errType: &ParseError{},
		},
		{
			name:    "rejects float64 with decimal",
			schema:  UintSchema{Min: 0, Max: 100},
			input:   50.5,
			wantErr: true,
			errType: &ParseError{},
		},
		{
			name:    "rejects invalid string",
			schema:  UintSchema{Min: 0, Max: 100},
			input:   "not a number",
			wantErr: true,
			errType: &ParseError{},
		},
		{
			name:    "rejects bool type",
			schema:  UintSchema{Min: 0, Max: 100},
			input:   true,
			wantErr: true,
			errType: &TypeError{},
		},
		{
			name:    "validates min constraint",
			schema:  UintSchema{Min: 10, Max: 100},
			input:   uint(5),
			wantErr: true,
			errType: &RangeError[uint]{},
		},
		{
			name:    "validates max constraint",
			schema:  UintSchema{Min: 0, Max: 100},
			input:   uint(150),
			wantErr: true,
			errType: &RangeError[uint]{},
		},
		{
			name:   "accepts value at min boundary",
			schema: UintSchema{Min: 10, Max: 100},
			input:  uint(10),
			want:   10,
		},
		{
			name:   "accepts value at max boundary",
			schema: UintSchema{Min: 0, Max: 100},
			input:  uint(100),
			want:   100,
		},
		{
			name:   "accepts zero when min is zero",
			schema: UintSchema{Min: 0, Max: 100},
			input:  uint(0),
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.schema.ParseAndValidate(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.IsType(t, tt.errType, err)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestUintSchema_TypeDescription(t *testing.T) {
	schema := UintSchema{Min: 1, Max: 500}
	desc := schema.TypeDescription()
	require.Contains(t, desc, "unsigned integer")
	require.Contains(t, desc, "1")
	require.Contains(t, desc, "500")
}
