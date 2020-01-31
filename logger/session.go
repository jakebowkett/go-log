package logger

import (
	"fmt"
)

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
End calls OnError and passes it a Thread containing only
the error level logs to Session.

If OnError or OnLog were nil nothing will happen.
*/
func (s *Session) End() {
	if s.ended {
		return
	}
	s.ended = true
	s.logger.end(kindSession, s.id, "", "", s.name, 0)
}
