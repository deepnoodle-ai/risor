package time

import (
	"context"
	"testing"
	"time"

	"github.com/deepnoodle-ai/wonton/assert"
	"github.com/risor-io/risor/object"
)

func TestNow(t *testing.T) {
	got := Now(context.Background())
	_, ok := got.(*object.Time)
	assert.True(t, ok)
}

func TestUnix(t *testing.T) {
	tests := []struct {
		sec  int64
		nsec int64
		want time.Time
	}{
		{0, 0, time.Unix(0, 0)},
		{1633046400, 0, time.Date(2021, 10, 1, 0, 0, 0, 0, time.UTC)},
		{1633046400, 500000000, time.Date(2021, 10, 1, 0, 0, 0, 500000000, time.UTC)},
	}

	for _, tt := range tests {
		got, _ := object.AsTime(
			Unix(context.Background(), object.NewInt(tt.sec), object.NewInt(tt.nsec)),
		)
		assert.Equal(t, got.UTC(), tt.want.UTC())
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		layout string
		value  string
		want   time.Time
	}{
		{time.RFC3339, "2021-10-01T15:30:45Z", time.Date(2021, 10, 1, 15, 30, 45, 0, time.UTC)},
		{time.RFC822, "01 Oct 21 15:30 UTC", time.Date(2021, 10, 1, 15, 30, 0, 0, time.UTC)},
		{time.Kitchen, "3:04PM", time.Date(0, 1, 1, 15, 4, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		got := Parse(context.Background(), object.NewString(tt.layout), object.NewString(tt.value))
		assert.Equal(t, got, object.NewTime(tt.want))
	}
}

func TestSince(t *testing.T) {
	now := time.Now()
	time.Sleep(100 * time.Millisecond)

	got := Since(context.Background(), object.NewTime(now))
	f, ok := got.(*object.Float)
	assert.True(t, ok)

	elapsed := f.Value()
	assert.True(t, elapsed >= 0.1)
	assert.True(t, elapsed < 0.25) // Allow some margin for error
}
