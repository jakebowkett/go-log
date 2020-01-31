package logger

import (
	"fmt"
	"strings"
	"time"
)

const timeFormat string = "MST 2006-01-02 15:04:05"

type Thread struct {
	Date     time.Time
	Kind     threadKind
	Id       string
	Ip       string
	Method   string
	Route    string
	Status   int
	Duration int64
	Entries  []*Entry
}

func (t Thread) FormatRecord() string {

	msg := ""
	for _, e := range t.Entries {
		if e.Message != "" {
			msg += e.Message + " "
		}
		if e.File != "" {
			fileParts := strings.SplitAfterN(e.File, "/storydevs", 2)
			file := fileParts[len(fileParts)-1]
			msg += fmt.Sprintf("%s:%d (%s)", file, e.Line, e.Function)
		}
		msg += "\n"
	}
	msg = strings.TrimSuffix(msg, "\n")

	s := ""
	switch t.Kind {
	case kindRequest:
		s = fmt.Sprintf(
			"%d %d %dms %s %s %s\n",
			t.Date.UnixNano(),
			t.Status,
			t.Duration/1000000,
			t.Method,
			t.Route,
			msg,
		)
	case kindSession:
		s = fmt.Sprintf(
			"%d %s\n",
			t.Date.UnixNano(),
			msg,
		)
	}

	return s
}

func (thread Thread) FormatTerse() string {

	var output string

	if thread.Kind == kindRequest {
		duration := fmt.Sprintf("%dms", thread.Duration/1000000)
		output = fmt.Sprintf(
			"%s %d %s %s\n",
			thread.Date.Format(time.Kitchen), thread.Status, duration, thread.Route)
	}

	if thread.Kind == kindSession {
		output = fmt.Sprintf(
			"%s Session: %s\n",
			thread.Date.Format(time.Kitchen), thread.Route)
	}

	for _, e := range thread.Entries {

		var kvs string
		for _, kv := range e.KeyVals {
			kvs += fmt.Sprintf("%q=%q", kv.Key, kv.Val)
		}

		output += fmt.Sprintf(
			"[%s] %s %s\n",
			e.Level, e.Message, kvs)
	}

	return output
}

func (thread Thread) FormatPretty() string {

	var output string

	if thread.Kind == kindRequest {

		duration := fmt.Sprintf("%dms", thread.Duration/1000000)
		duration = pad(duration, 10)

		lastColon := strings.LastIndex(thread.Ip, ":")
		if lastColon == -1 {
			lastColon = 0
		}
		ip := thread.Ip[0:lastColon]

		output = fmt.Sprintf(
			// "\nRequest: %s, IPs: %s"+
			"\n%s %d %s %s %s %s\n",
			// thread.Id,
			thread.Date.Format(time.Kitchen),
			thread.Status,
			pad(ip, 20),
			duration,
			thread.Method,
			thread.Route)
	}

	if thread.Kind == kindSession {
		if thread.Route == "" {
			output = "\n" + thread.Date.Format(time.Kitchen) + "\n"
		} else {
			output = fmt.Sprintf(
				// "\nEntry: %s"+
				"\n%s Session: %s\n",
				// thread.Id,
				thread.Date.Format(time.Kitchen),
				thread.Route)
		}
	}

	for i, e := range thread.Entries {

		lnStart := "├─"
		fStart := "│ "
		if i == len(thread.Entries)-1 {
			lnStart = "└─"
			fStart = "  "
		}

		fileParts := strings.SplitAfterN(e.File, "/storydevs", 2)
		file := fileParts[len(fileParts)-1]

		// We quote strings since they might have spaces.
		var kvs string
		for _, kv := range e.KeyVals {

			var val string
			switch kv.Val.(type) {
			case error:
				val = fmt.Sprintf("\"%v\"", kv.Val.(error).Error())
			case string:
				val = fmt.Sprintf("\"%v\"", kv.Val)
			default:
				val = fmt.Sprintf("%v", kv.Val)
			}

			kvs += fmt.Sprintf(" %s    %s = %s\n", fStart, kv.Key, val)
		}

		var runtimeInfo string
		if file != "" {
			runtimeInfo = fmt.Sprintf(
				" %s %s:%d (%s)\n",
				fStart, file, e.Line, e.Function)
		}

		msgParts := strings.Split(e.Message, "\n")
		for i := range msgParts {
			if i == 0 {
				continue
			}
			msgParts[i] = fmt.Sprintf(" %s    %s", fStart, msgParts[i])
		}

		output += fmt.Sprintf(
			" │\n"+
				" %s [%s] %s\n"+
				"%s"+
				"%s",
			lnStart, e.Level, strings.Join(msgParts, "\n"), kvs, runtimeInfo)
	}

	return output
}

func pad(s string, length int) string {
	diff := length - len([]rune(s))
	if diff <= 0 {
		return s
	}
	return strings.Repeat("_", diff) + s
}
