package logger

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	levelInfo  = logLevel{"Info"}
	levelError = logLevel{"Error"}
	levelDebug = logLevel{"Debug"}

	kindRequest = threadKind{"request"}
	kindSession = threadKind{"session"}
)

type logLevel struct {
	name string
}

func (ll logLevel) String() string {
	return ll.name
}

type threadKind struct {
	name string
}

func (tk threadKind) String() string {
	return tk.name
}

type HeaderWriter interface {
	WriteHeader(int)
}

type kv struct {
	Key string
	Val interface{}
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

func (e *Entry) Data(k string, v interface{}) *Entry {
	e.KeyVals = append(e.KeyVals, kv{k, v})
	return e
}

func (e *Entry) DataMulti(kvs KeyValuer) *Entry {
	for k, v, done := kvs.Next(); !done; {
		e.KeyVals = append(e.KeyVals, kv{k, v})
	}
	return e
}

type KeyValuer interface {
	Next() (key string, val interface{}, done bool)
}

type Logger struct {
	OnLog     func(Thread)
	OnError   func(Thread)
	idCount   int64
	debug     bool
	runtime   bool
	idCountMu sync.Mutex
	debugMu   sync.Mutex
	runtimeMu sync.Mutex
	logs      sync.Map
}

func (l *Logger) SetDebug(enabled bool) {
	l.debugMu.Lock()
	l.debug = enabled
	l.debugMu.Unlock()
}
func (l *Logger) SetRuntime(enabled bool) {
	l.runtimeMu.Lock()
	l.runtime = enabled
	l.runtimeMu.Unlock()
}

/*
NewId generates a new id to associate with a particular
log thread or session thread. It increments numerical
ids, starting from 1.
*/

func (l *Logger) NewId() string {
	l.idCountMu.Lock()

	// We defer to avoid idCount changing between
	// incrementing it and converting it to a string.
	defer l.idCountMu.Unlock()
	l.idCount++
	return strconv.FormatInt(l.idCount, 10)
}

func (l *Logger) HttpStatus(reqId string, w HeaderWriter, code int) {
	l.logStatus(reqId, w, code)
}
func (l *Logger) Redirect(reqId string, code int) {
	l.logs.Store(reqId+"_status", code)
}
func (l *Logger) BadRequest(reqId string, w HeaderWriter, msg string) *Entry {
	l.logStatus(reqId, w, 400)
	return l.logEntry(levelError, reqId, msg)
}
func (l *Logger) Unauthorised(reqId string, w HeaderWriter) {
	l.logStatus(reqId, w, 401)
}
func (l *Logger) NotFound(reqId string, w HeaderWriter) {
	l.logStatus(reqId, w, 404)
}
func (l *Logger) logStatus(reqId string, w HeaderWriter, code int) {
	w.WriteHeader(code)
	l.logs.Store(reqId+"_status", code)
}

func (l *Logger) ErrorMulti(reqId, msg, key string, errs []error) *Entry {

	m := map[string]int{}
	for _, err := range errs {
		if err == nil {
			continue
		}
		m[err.Error()]++
	}

	e := l.logEntry(levelError, reqId, msg)

	for es, i := range m {
		e.Data(key, fmt.Sprintf("(%d instances) %s", i, es))
	}

	return e
}

func (l *Logger) Fatal(err error) {
	id := l.NewId()
	l.logEntry(levelError, id, err.Error())
	l.end(kindSession, id, "", "", "", 0)
	os.Exit(1)
}

func (l *Logger) Once(msg string) {
	id := l.NewId()
	l.logEntry(levelInfo, id, msg)
	l.end(kindSession, id, "", "", "", 0)
}
func (l *Logger) OnceF(format string, a ...interface{}) {
	id := l.NewId()
	l.logEntry(levelInfo, id, fmt.Sprintf(format, a...))
	l.end(kindSession, id, "", "", "", 0)
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

func (l *Logger) End(reqId, ip, method, route string, duration int64) {
	l.end(kindRequest, reqId, ip, method, route, duration)
}

func (l *Logger) logEntry(level logLevel, threadId, msg string) *Entry {

	// Capitalise msg and add a period at the end.
	if !strings.HasSuffix(msg, ".") {
		msg += "."
	}
	for _, r := range msg {
		msg = strings.ToUpper(string(r)) + msg[len(string(r)):]
		break
	}

	if level == levelDebug && !l.debug {
		return &Entry{}
	}

	e := &Entry{
		ThreadId: threadId,
		Level:    level.String(),
		Message:  msg,
	}

	if l.runtime {
		function, file, line := callSite()
		e.Function = function
		e.File = file
		e.Line = line
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

func (l *Logger) end(kind threadKind, threadId, ip, method, route string, duration int64) {

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

	log := Thread{
		Date:     time.Now(),
		Id:       threadId,
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

	if l.OnLog == nil {
		return
	}
	l.OnLog(log)
}

func (l *Logger) status(reqId string) (code int) {
	status, ok := l.logs.Load(reqId + "_status")
	if ok {
		return status.(int)
	}
	return 200
}

func callSite() (string, string, int) {

	pc, fn, ln, ok := runtime.Caller(3)
	if !ok {
		return "Unknown", "Unable to obtain call site", 0
	}

	function := runtime.FuncForPC(pc).Name()
	if idx := strings.LastIndex(function, "/"); idx != -1 {
		function = function[idx+1 : len(function)]
	}

	return function, fn, ln
}
