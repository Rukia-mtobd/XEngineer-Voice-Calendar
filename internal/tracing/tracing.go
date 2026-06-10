package tracing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	serviceName   = "xengineer-voice-calendar"
	traceFileName = "voice_calendar_trace.jsonl"
	maxAttrLen    = 8192
)

var (
	mu       sync.RWMutex
	enabled  bool
	tracer   oteltrace.Tracer
	shutdown func(context.Context) error
)

// Init 初始化 OpenTelemetry Tracer，Span 追加写入程序目录下 voice_calendar_trace.jsonl。
func Init() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	tracePath := filepath.Join(filepath.Dir(exe), traceFileName)

	exporter, err := newJSONLExporter(tracePath)
	if err != nil {
		return fmt.Errorf("open trace file: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			"",
			attribute.String("service.name", serviceName),
		)),
	)
	otel.SetTracerProvider(tp)

	mu.Lock()
	enabled = true
	tracer = tp.Tracer(serviceName)
	shutdown = tp.Shutdown
	mu.Unlock()

	fmt.Println("tracing enabled, log file:", tracePath)
	return nil
}

// Shutdown 刷盘并关闭 Tracer。
func Shutdown(ctx context.Context) error {
	mu.RLock()
	fn := shutdown
	mu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

// Enabled 是否已启用 tracing。
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled
}

// StartSpan 开始一个 Span；未启用时返回原 context 与 noop span。
func StartSpan(ctx context.Context, name string) (context.Context, oteltrace.Span) {
	mu.RLock()
	t := tracer
	ok := enabled
	mu.RUnlock()
	if !ok || t == nil {
		return ctx, oteltrace.SpanFromContext(ctx)
	}
	return t.Start(ctx, name)
}

// Event 在当前 Span 上记录事件（用于标记 LLM 原始返回、JSON 提取等步骤）。
func Event(ctx context.Context, name string, attrs map[string]string) {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return
	}
	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kvs = append(kvs, attribute.String(k, truncate(v)))
	}
	span.AddEvent(name, oteltrace.WithAttributes(kvs...))
}

// SetAttrs 为当前 Span 设置属性。
func SetAttrs(ctx context.Context, attrs map[string]string) {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return
	}
	kvs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kvs = append(kvs, attribute.String(k, truncate(v)))
	}
	span.SetAttributes(kvs...)
}

// RecordError 记录错误并标记 Span 失败。
func RecordError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	span := oteltrace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func truncate(s string) string {
	if len(s) <= maxAttrLen {
		return s
	}
	return s[:maxAttrLen] + "...(truncated)"
}
