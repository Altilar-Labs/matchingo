package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	// RequestIDKey is the key used to store request IDs in context
	RequestIDKey contextKey = "request_id"
)

// Config defines logging configuration
type Config struct {
	// Level is the logging level (debug, info, warn, error)
	Level string
	// Pretty determines if logs should be formatted for human readability
	Pretty bool
	// Output is where logs are written (defaults to os.Stdout)
	Output io.Writer
}

// DefaultConfig returns the default logging configuration
func DefaultConfig() Config {
	return Config{
		Level:  "info",
		Pretty: false,
		Output: os.Stdout,
	}
}

// Setup configures global logging based on the provided config
func Setup(cfg Config) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output
	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	// Set up pretty logging if enabled
	if cfg.Pretty {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
		}
	}

	// Set global logger
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

// FromContext extracts a logger with request context
func FromContext(ctx context.Context) zerolog.Logger {
	// Extract request ID if present
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return log.With().Str("request_id", requestID).Logger()
	}

	// Extract metadata from gRPC context
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// Add metadata fields to logger
		logCtx := log.With()
		for k, v := range md {
			if len(v) > 0 {
				logCtx = logCtx.Str(k, v[0])
			}
		}
		return logCtx.Logger()
	}

	return log.Logger
}

// LoggingInterceptor returns a gRPC interceptor for request logging
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Set up logger with method info
		logger := log.With().
			Str("grpc.method", info.FullMethod).
			Logger()

		// Extract metadata
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if requestIDs := md.Get("x-request-id"); len(requestIDs) > 0 {
				requestID := requestIDs[0]
				logger = logger.With().Str("request_id", requestID).Logger()
				ctx = context.WithValue(ctx, RequestIDKey, requestID)
			}
		}

		// Log request
		logger.Debug().Msg("Request received")

		// Process the request
		resp, err := handler(ctx, req)

		// Calculate duration
		duration := time.Since(start)

		// Get status code
		statusCode := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			} else {
				statusCode = codes.Unknown
			}
		}

		// Log response with appropriate level based on status
		logEvent := logger.Info()
		if statusCode != codes.OK {
			logEvent = logger.Error().Err(err).Str("grpc.code", statusCode.String())
		}

		logEvent.Dur("duration", duration).
			Int("grpc.status", int(statusCode)).
			Msg(fmt.Sprintf("Request completed in %v", duration))

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC interceptor for streaming request logging
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		// Create a context-aware stream wrapper
		wrappedStream := &wrappedServerStream{
			ServerStream: stream,
			ctx:          stream.Context(),
		}

		// Set up logger with method info
		logger := log.With().
			Str("grpc.method", info.FullMethod).
			Bool("grpc.stream", true).
			Logger()

		// Extract metadata
		if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
			if requestIDs := md.Get("x-request-id"); len(requestIDs) > 0 {
				requestID := requestIDs[0]
				logger = logger.With().Str("request_id", requestID).Logger()
				wrappedStream.ctx = context.WithValue(wrappedStream.ctx, RequestIDKey, requestID)
			}
		}

		// Log stream start
		logger.Debug().Msg("Stream started")

		// Process the stream
		err := handler(srv, wrappedStream)

		// Calculate duration
		duration := time.Since(start)

		// Get status code
		statusCode := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			} else {
				statusCode = codes.Unknown
			}
		}

		// Log stream end with appropriate level based on status
		logEvent := logger.Info()
		if statusCode != codes.OK {
			logEvent = logger.Error().Err(err).Str("grpc.code", statusCode.String())
		}

		logEvent.Dur("duration", duration).
			Int("grpc.status", int(statusCode)).
			Msg(fmt.Sprintf("Stream completed in %v", duration))

		return err
	}
}

// wrappedServerStream wraps a grpc.ServerStream with a modified context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapper's modified context
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
