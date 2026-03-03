package logger

import (
	"log"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type (
	responseData struct {
		size       int
		statuscode int
	}

	LoggingResponseWriter struct {
		http.ResponseWriter
		responseData
	}

	Logger struct {
		*zap.SugaredLogger
	}
)

var sugar *Logger

func (l *LoggingResponseWriter) Write(bytes []byte) (int, error) {
	size, err := l.ResponseWriter.Write(bytes)
	l.responseData.size += size
	return size, err
}

func (l *LoggingResponseWriter) WriteHeader(statuscode int) {
	l.ResponseWriter.WriteHeader(statuscode)
	l.responseData.statuscode = statuscode
}

func NewLogger() *Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("failed create logger", err)
		return nil
	}
	defer logger.Sync()
	sugar = &Logger{
		SugaredLogger: logger.Sugar(),
	}
	return sugar
}

func WithLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lw := LoggingResponseWriter{ResponseWriter: w, responseData: responseData{}}

		h.ServeHTTP(&lw, r)
		duration := time.Since(start)
		sugar.Infoln(
			"\n",
			"-----REQUEST-----\n",
			"URI:", r.RequestURI, "\n",
			"Method:", r.Method, "\n",
			"Duration:", duration, "\n",
			"-----RESPONSE-----\n",
			"Status:", lw.statuscode, "\n",
			"Size:", lw.size, "\n",
		)
	})
}
