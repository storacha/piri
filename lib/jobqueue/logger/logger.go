package logger

// subset from ipfs go-log v2
type StandardLogger interface {
	Debug(args ...interface{})
	Debugw(msg string, keysAndValues ...interface{})
	Debugf(format string, args ...interface{})

	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Infow(msg string, keysAndValues ...interface{})

	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Warnw(msg string, keysAndValues ...interface{})

	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Errorw(msg string, keysAndValues ...interface{})
}

type DiscardLogger struct{}

var _ StandardLogger = (*DiscardLogger)(nil)

func (d *DiscardLogger) Debug(args ...interface{})                       {}
func (d *DiscardLogger) Debugf(format string, args ...interface{})       {}
func (d *DiscardLogger) Debugw(msg string, keysAndValues ...interface{}) {}
func (d *DiscardLogger) Error(args ...interface{})                       {}
func (d *DiscardLogger) Errorf(format string, args ...interface{})       {}
func (d *DiscardLogger) Errorw(msg string, keysAndValues ...interface{}) {}
func (d *DiscardLogger) Infof(format string, args ...interface{})        {}
func (d *DiscardLogger) Info(args ...interface{})                        {}
func (d *DiscardLogger) Infow(msg string, keysAndValues ...interface{})  {}
func (d *DiscardLogger) Warn(args ...interface{})                        {}
func (d *DiscardLogger) Warnf(format string, args ...interface{})        {}
func (d *DiscardLogger) Warnw(msg string, keysAndValues ...interface{})  {}
