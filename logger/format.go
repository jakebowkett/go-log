package logger

import (
	"fmt"
	"strings"
	"time"
)

const timeFormat string = "MST 2006-01-02 15:04:05"

func pad(s string, length int) string {
	diff := length - len([]rune(s))
	if diff <= 0 {
		return s
	}
	return strings.Repeat("_", diff) + s
}

func (log Log) FormatRecord() string {
	return ""
}

func (log Log) FormatTerse() string {

	var output string

	if log.Kind == kindRequest {
		duration := fmt.Sprintf("%dms", log.Duration/1000000)
		output = fmt.Sprintf(
			"%s %d %s %s\n",
			log.Date.Format(time.Kitchen), log.Status, duration, log.Route)
	}

	if log.Kind == kindSession {
		output = fmt.Sprintf(
			"%s Session: %s\n",
			log.Date.Format(time.Kitchen), log.Route)
	}

	for _, e := range log.Entries {

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

func (log Log) FormatPretty() string {

	var output string

	if log.Kind == kindRequest {
		duration := fmt.Sprintf("%dms", log.Duration/1000000)
		duration = pad(duration, 10)
		output = fmt.Sprintf(
			// "\nRequest: %s, IPs: %s"+
			"\n%s %d %s %s %s\n",
			// log.ThreadId,
			// log.Ip,
			log.Date.Format(time.Kitchen),
			log.Status,
			duration,
			log.Method,
			log.Route)
	}

	if log.Kind == kindSession {
		output = fmt.Sprintf(
			// "\nEntry: %s"+
			"\n%s Session: %s\n",
			// log.ThreadId,
			log.Date.Format(time.Kitchen),
			log.Route)
	}

	for i, e := range log.Entries {

		lnStart := "├─"
		fStart := "│ "
		if i == len(log.Entries)-1 {
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
				val = fmt.Sprintf("%s", kv.Val.(error).Error())
			case string:
				val = fmt.Sprintf("%s", kv.Val)
			default:
				val = fmt.Sprintf("%v", kv.Val)
			}

			kvs += fmt.Sprintf(" %s    %s = %s\n", fStart, kv.Key.String(), val)
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
