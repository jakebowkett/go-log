package log

import "fmt"

type Entry interface {
	Data(k fmt.Stringer, v interface{}) Entry
}

// type Logger interface {
// 	Info(reqId string, msg string) Entry
// 	Error(reqId string, msg string) Entry
// 	Debug(reqId string, msg string) Entry

// 	InfoF(reqId string, msg string, a ...interface{}) Entry
// 	ErrorF(reqId string, msg string, a ...interface{}) Entry
// 	DebugF(reqId string, msg string, a ...interface{}) Entry

// 	End(reqId, route string, status, duration int)

// 	Sess(name string) Session
// }

type Session interface {
	Info(msg string) Entry
	Error(msg string) Entry
	Debug(msg string) Entry

	InfoF(format string, a ...interface{}) Entry
	ErrorF(format string, a ...interface{}) Entry
	DebugF(format string, a ...interface{}) Entry

	End()
}
