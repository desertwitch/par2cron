package logging

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

// recordHandler is a minimal slog.Handler that records calls for testing.
type recordHandler struct {
	enabled bool
	records []slog.Record
	attrs   []slog.Attr
	groups  []string
}

func (h *recordHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return h.enabled
}

func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)

	return nil
}

func (h *recordHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))

	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &recordHandler{enabled: h.enabled, attrs: newAttrs}
}

func (h *recordHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)

	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &recordHandler{enabled: h.enabled, groups: newGroups}
}

// Expectation: Enabled should return true if at least one handler is enabled.
func Test_FanoutHandler_Enabled_OneEnabled(t *testing.T) {
	t.Parallel()

	f := &fanoutHandler{
		handlers: []slog.Handler{
			&recordHandler{enabled: false},
			&recordHandler{enabled: true},
		},
	}

	require.True(t, f.Enabled(context.Background(), slog.LevelInfo))
}

// Expectation: Enabled should return false if no handlers are enabled.
func Test_FanoutHandler_Enabled_NoneEnabled(t *testing.T) {
	t.Parallel()

	f := &fanoutHandler{
		handlers: []slog.Handler{
			&recordHandler{enabled: false},
			&recordHandler{enabled: false},
		},
	}

	require.False(t, f.Enabled(context.Background(), slog.LevelInfo))
}

// Expectation: Handle should fan out to all enabled handlers only.
func Test_FanoutHandler_Handle_FansOutToEnabled(t *testing.T) {
	t.Parallel()

	enabled := &recordHandler{enabled: true}
	disabled := &recordHandler{enabled: false}
	f := &fanoutHandler{
		handlers: []slog.Handler{enabled, disabled},
	}

	r2 := slog.Record{}
	r2.Level = slog.LevelInfo
	r2.Message = "test"
	err := f.Handle(context.Background(), r2)

	require.NoError(t, err)
	require.Len(t, enabled.records, 1)
	require.Empty(t, disabled.records)
}

// Expectation: Handle should deliver to multiple enabled handlers.
func Test_FanoutHandler_Handle_MultipleEnabled(t *testing.T) {
	t.Parallel()

	h1 := &recordHandler{enabled: true}
	h2 := &recordHandler{enabled: true}
	f := &fanoutHandler{
		handlers: []slog.Handler{h1, h2},
	}

	var r slog.Record
	r.Level = slog.LevelWarn
	r.Message = "warning"
	err := f.Handle(context.Background(), r)

	require.NoError(t, err)
	require.Len(t, h1.records, 1)
	require.Len(t, h2.records, 1)
}

// Expectation: WithAttrs should return a new fanoutHandler with attrs applied to all children.
func Test_FanoutHandler_WithAttrs_PropagatesAttrs(t *testing.T) {
	t.Parallel()

	h1 := &recordHandler{enabled: true}
	h2 := &recordHandler{enabled: true}
	f := &fanoutHandler{
		handlers: []slog.Handler{h1, h2},
	}

	attrs := []slog.Attr{slog.String("key", "value")}
	newHandler := f.WithAttrs(attrs)
	newFanout, ok := newHandler.(*fanoutHandler)
	require.True(t, ok)
	require.Len(t, newFanout.handlers, 2)

	for _, h := range newFanout.handlers {
		rh, ok := h.(*recordHandler)
		require.True(t, ok)
		require.Len(t, rh.attrs, 1)
		require.Equal(t, "key", rh.attrs[0].Key)
	}
}

// Expectation: WithGroup should return a new fanoutHandler with group applied to all children.
func Test_FanoutHandler_WithGroup_PropagatesGroup(t *testing.T) {
	t.Parallel()

	h1 := &recordHandler{enabled: true}
	h2 := &recordHandler{enabled: true}
	f := &fanoutHandler{
		handlers: []slog.Handler{h1, h2},
	}

	newHandler := f.WithGroup("mygroup")
	newFanout, ok := newHandler.(*fanoutHandler)
	require.True(t, ok)
	require.Len(t, newFanout.handlers, 2)

	for _, h := range newFanout.handlers {
		rh, ok := h.(*recordHandler)
		require.True(t, ok)
		require.Len(t, rh.groups, 1)
		require.Equal(t, "mygroup", rh.groups[0])
	}
}

// Expectation: Enabled with empty handlers should return false.
func Test_FanoutHandler_Enabled_NoHandlers(t *testing.T) {
	t.Parallel()

	f := &fanoutHandler{handlers: []slog.Handler{}}
	require.False(t, f.Enabled(context.Background(), slog.LevelInfo))
}

// Expectation: Handle with empty handlers should succeed without error.
func Test_FanoutHandler_Handle_NoHandlers(t *testing.T) {
	t.Parallel()

	f := &fanoutHandler{handlers: []slog.Handler{}}

	var r slog.Record
	err := f.Handle(context.Background(), r)
	require.NoError(t, err)
}
