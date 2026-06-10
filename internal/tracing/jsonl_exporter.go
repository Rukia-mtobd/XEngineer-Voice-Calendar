package tracing

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
)

// jsonlExporter 将 Span 写入 JSONL 文件，便于本地追溯 LLM 解析链路。
type jsonlExporter struct {
	file *os.File
	mu   sync.Mutex
}

type spanRecord struct {
	Name       string            `json:"name"`
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_span_id,omitempty"`
	Start      time.Time         `json:"start"`
	End        time.Time         `json:"end"`
	DurationMs int64             `json:"duration_ms"`
	Status     string            `json:"status"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Events     []eventRecord     `json:"events,omitempty"`
}

type eventRecord struct {
	Name       string            `json:"name"`
	Time       time.Time         `json:"time"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func newJSONLExporter(path string) (*jsonlExporter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &jsonlExporter{file: f}, nil
}

func (e *jsonlExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	_ = ctx
	records := make([]spanRecord, 0, len(spans))
	for _, sp := range spans {
		rec := spanRecord{
			Name:       sp.Name(),
			TraceID:    sp.SpanContext().TraceID().String(),
			SpanID:     sp.SpanContext().SpanID().String(),
			Start:      sp.StartTime(),
			End:        sp.EndTime(),
			DurationMs: sp.EndTime().Sub(sp.StartTime()).Milliseconds(),
			Status:     sp.Status().Code.String(),
			Attributes: attrsToMap(sp.Attributes()),
		}
		if p := sp.Parent().SpanID(); p.IsValid() {
			rec.ParentID = p.String()
		}
		for _, ev := range sp.Events() {
			rec.Events = append(rec.Events, eventRecord{
				Name:       ev.Name,
				Time:       ev.Time,
				Attributes: attrsToMap(ev.Attributes),
			})
		}
		records = append(records, rec)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for _, rec := range records {
		b, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		if _, err := e.file.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func (e *jsonlExporter) Shutdown(ctx context.Context) error {
	_ = ctx
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.file == nil {
		return nil
	}
	err := e.file.Close()
	e.file = nil
	return err
}

func attrsToMap(attrs []attribute.KeyValue) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	m := make(map[string]string, len(attrs))
	for _, a := range attrs {
		m[string(a.Key)] = a.Value.AsString()
	}
	return m
}
