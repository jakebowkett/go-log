package logger

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

type logLevel struct {
	name string
}

func (ll logLevel) String() string {
	return ll.name
}

var (
	levelStatus = logLevel{"Status"}
	levelInfo   = logLevel{"Info"}
	levelError  = logLevel{"Error"}
	levelDebug  = logLevel{"Debug"}
)

type logKind struct {
	name string
}

func (lk logKind) String() string {
	return lk.name
}

var (
	kindRequest = logKind{"request"}
	kindSession = logKind{"session"}
)

type Log struct {
	Date     time.Time
	Kind     logKind
	ThreadId string
	Ip       string
	Method   string
	Route    string
	Status   int
	Duration int
	Entries  []*Entry
}

type Entry struct {
	ThreadId string
	Level    string
	Function string
	File     string
	Message  string
	Line     int
	KeyVals  []kv
}

func (e *Entry) Data(k fmt.Stringer, v interface{}) *Entry {
	e.KeyVals = append(e.KeyVals, kv{k, v})
	return e
}

type kv struct {
	Key fmt.Stringer
	Val interface{}
}

type Logger struct {
	OnLogEvent     func(Log)
	OnError        func(Log)
	IdGenerator    func() (string, error)
	DisableDebug   bool
	DisableRuntime bool
	logs           sync.Map
	idCountMu      sync.Mutex
	idCount        int
}

type HeaderWriter interface {
	WriteHeader(int)
}

func (l *Logger) BadRequest(reqId string, w HeaderWriter, msg string) {
	l.logStatus(reqId, w, 400)
	l.logEntry(levelError, reqId, msg)
}
func (l *Logger) NotFound(reqId string, w HeaderWriter) {
	l.logStatus(reqId, w, 404)
}
func (l *Logger) logStatus(reqId string, w HeaderWriter, code int) {
	w.WriteHeader(code)
	l.logs.Store(reqId+"_status", code)
}

func (l *Logger) Info(reqId, msg string) *Entry {
	return l.logEntry(levelInfo, reqId, msg)
}
func (l *Logger) Error(reqId, msg string) *Entry {
	return l.logEntry(levelError, reqId, msg)
}
func (l *Logger) Debug(reqId, msg string) *Entry {
	return l.logEntry(levelDebug, reqId, msg)
}

func (l *Logger) InfoF(reqId, format string, a ...interface{}) *Entry {
	return l.logEntry(levelInfo, reqId, fmt.Sprintf(format, a...))
}
func (l *Logger) ErrorF(reqId, format string, a ...interface{}) *Entry {
	return l.logEntry(levelError, reqId, fmt.Sprintf(format, a...))
}
func (l *Logger) DebugF(reqId, format string, a ...interface{}) *Entry {
	return l.logEntry(levelDebug, reqId, fmt.Sprintf(format, a...))
}

func (l *Logger) End(reqId, ip, method, route string, duration int) {
	l.end(kindRequest, reqId, ip, method, route, duration)
}

/*
NewId generates a new id to associate with a particular
log thread or session thread. If a function has not been
supplied to Logger.IdGenerator it defaults to incrementing
numerical ids, starting from 1.
*/
func (l *Logger) NewId() string {

	if l.IdGenerator != nil {

		id, err := l.IdGenerator()
		if err != nil {
			l.logIdGenError(err)
			return l.defaultId()
		}
		return id
	}

	return l.defaultId()
}

func (l *Logger) defaultId() string {
	l.idCountMu.Lock()
	l.idCount++
	id := l.idCount
	l.idCountMu.Unlock()
	return fmt.Sprintf("%d", id)
}

func (l *Logger) logIdGenError(err error) {
	e := &Entry{
		Level:   levelError.String(),
		Message: "couldn't generate ID for logger thread: " + err.Error(),
	}
	l.insertEntry(e)
}

func (l *Logger) getCallSite() (string, string, int) {

	var file string
	var function string
	var line int

	if l.DisableRuntime {
		return function, file, line
	}

	pc, fn, ln, ok := runtime.Caller(3)
	if !ok {
		file = "Unable to obtain call site."
		function = "Unknown"
	} else {
		function = runtime.FuncForPC(pc).Name()
		if idx := strings.LastIndex(function, "/"); idx != -1 {
			function = function[idx+1 : len(function)]
		}
		file = fn
	}
	line = ln

	return function, file, line
}

func (l *Logger) logEntry(level logLevel, threadId, msg string) *Entry {

	if level == levelDebug && l.DisableDebug {
		return &Entry{}
	}

	function, file, line := l.getCallSite()

	e := &Entry{
		ThreadId: threadId,
		Level:    level.String(),
		Function: function,
		File:     file,
		Line:     line,
		Message:  msg,
	}

	l.insertEntry(e)

	return e
}

func (l *Logger) insertEntry(e *Entry) {

	entries, ok := l.logs.Load(e.ThreadId)
	if !ok {
		l.logs.Store(e.ThreadId, []*Entry{e})
		return
	}

	// We know the map only has this type as values.
	ee := entries.([]*Entry)
	ee = append(ee, e)
	l.logs.Store(e.ThreadId, ee)
}

func (l *Logger) status(reqId string) (code int) {
	status, ok := l.logs.Load(reqId + "_status")
	if ok {
		return status.(int)
	}
	return 200
}

func (l *Logger) end(kind logKind, threadId, ip, method, route string, duration int) {

	var ee []*Entry
	entries, ok := l.logs.Load(threadId)
	if ok {
		l.logs.Delete(threadId)
		ee = entries.([]*Entry)
	}

	// Unlike requests there's no value in logging a
	// session with no entries because it doesn't have
	// an overall HTTP status or duration to report.
	if kind == kindSession && len(ee) == 0 {
		return
	}

	log := Log{
		Date:     time.Now(),
		ThreadId: threadId,
		Kind:     kind,
		Ip:       ip,
		Method:   method,
		Route:    route,
		Duration: duration,
		Entries:  ee,
	}

	if kind == kindRequest {
		log.Status = l.status(threadId)
	}

	if l.OnError != nil {
		var errs []*Entry
		for _, e := range ee {
			if e.Level == levelError.String() {
				errs = append(errs, e)
			}
		}
		if errs != nil {
			log.Entries = errs
			l.OnError(log)
		}
	}

	if l.OnLogEvent == nil {
		return
	}
	l.OnLogEvent(log)
}

type Session struct {
	logger *Logger
	name   string
	id     string
	ended  bool
}

func (l *Logger) Sess(name string) *Session {
	return &Session{
		id:     l.NewId(),
		name:   name,
		logger: l,
	}
}

func (s *Session) SeenError() bool {

	var ee []*Entry
	entries, ok := s.logger.logs.Load(s.id)
	if !ok {
		return false
	}
	ee = entries.([]*Entry)

	for _, e := range ee {
		if e.Level == "Error" {
			return true
		}
	}
	return false
}

func (s *Session) Info(msg string) *Entry {
	if s.ended {
		return &Entry{}
	}
	return s.logger.logEntry(levelInfo, s.id, msg)
}
func (s *Session) Error(msg string) *Entry {
	if s.ended {
		return &Entry{}
	}
	return s.logger.logEntry(levelError, s.id, msg)
}
func (s *Session) Debug(msg string) *Entry {
	if s.ended {
		return &Entry{}
	}
	return s.logger.logEntry(levelDebug, s.id, msg)
}

func (s *Session) InfoF(format string, a ...interface{}) *Entry {
	if s.ended {
		return &Entry{}
	}
	return s.logger.logEntry(levelInfo, s.id, fmt.Sprintf(format, a...))
}
func (s *Session) ErrorF(format string, a ...interface{}) *Entry {
	if s.ended {
		return &Entry{}
	}
	return s.logger.logEntry(levelError, s.id, fmt.Sprintf(format, a...))
}
func (s *Session) DebugF(format string, a ...interface{}) *Entry {
	if s.ended {
		return &Entry{}
	}
	return s.logger.logEntry(levelDebug, s.id, fmt.Sprintf(format, a...))
}

/*
End calls OnError and passes it a Log containing only
the error level logs to Session.

If OnError or OnLogEvent were nil nothing will happen.
*/
func (s *Session) End() {
	if s.ended {
		return
	}
	s.ended = true
	s.logger.end(kindSession, s.id, "", "", s.name, 0)
}
