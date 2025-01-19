package otel

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	otlpLog "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	otelLog "go.opentelemetry.io/otel/log"
	otelLogSdk "go.opentelemetry.io/otel/sdk/log"
)

var logger otelLog.Logger
var done bool

func InitLogger() error {
	service := "http-proxy-lantern"
	expLog, err := otlpLog.New(context.Background(),
		otlpLog.WithEndpoint("172.16.0.88:4318"),
		// otlpLog.WithTLSClientConfig(&tls.Config{InsecureSkipVerify: true}),
		otlpLog.WithInsecure(),
	)
	if err != nil {
		return err
	}

	resourceAttributes := []attribute.KeyValue{
		semconv.ServiceNameKey.String(service),
	}

	r := resource.NewWithAttributes(semconv.SchemaURL, resourceAttributes...)

	provider := otelLogSdk.NewLoggerProvider(
		otelLogSdk.WithProcessor(otelLogSdk.NewBatchProcessor(expLog)),
		otelLogSdk.WithResource(r),
	)

	logger = provider.Logger(service)
	done = true
	return nil
}

func Debug(ctx context.Context, title string) {
	if !done {
		InitLogger() // For now I want to see if I can get logs to the otel collector
	}
	var record otelLog.Record
	record.SetTimestamp(time.Now())
	record.SetBody(otelLog.StringValue(title))
	record.SetSeverity(otelLog.SeverityDebug)
	record.SetSeverityText(otelLog.SeverityDebug.String())

	logger.Emit(ctx, record)
}
