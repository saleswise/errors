// This module implements functions which manipulate errors and provide stack
// trace information.
//
// NOTE: This package intentionally mirrors the standard "errors" module.
// All dropbox code should use this.
package errors

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"encoding/json"
)

// This interface exposes additional information about the error.
type DropboxError interface {
	// This returns the error message without the stack trace.
	GetMessage() string

	// This returns the stack trace without the error message.
	GetStack() string

	// This returns the stack trace's context.
	GetContext() string

	// This returns the wrapped error.  This returns nil if this does not wrap
	// another error.
	GetInner() error

	// This checks all transitive inners for the specified error.
	HasInner(e error) bool

	// Implements the built-in error interface.
	Error() string

	// This sets the state of the error, as decided by the creator.
	SetState(state map[string]interface{}) DropboxError

	// This returns the state of the error.
	GetState() map[string]interface{}

	// This returns the state of the error and all inner errors.
	GetAnnotatedStates() []map[string]interface{}
}

// Standard struct for general types of errors.
//
// For an example of custom error type, look at databaseError/newDatabaseError
// in errors_test.go.
type DropboxBaseError struct {
	Msg     string
	Stack   string
	Context string
	State   map[string]interface{}
	inner   error
}

// This returns the error string without stack trace information.
func GetMessage(err interface{}) string {
	switch e := err.(type) {
	case DropboxError:
		dberr := DropboxError(e)
		ret := []string{}
		for dberr != nil {
			ret = append(ret, dberr.GetMessage())
			d := dberr.GetInner()
			if d == nil {
				break
			}
			var ok bool
			dberr, ok = d.(DropboxError)
			if !ok {
				ret = append(ret, d.Error())
				break
			}
		}
		return strings.Join(ret, " ")
	case runtime.Error:
		return runtime.Error(e).Error()
	default:
		return "Passed a non-error to GetMessage"
	}
}

// This returns a string with all available error information, including inner
// errors that are wrapped by this errors.
func (e *DropboxBaseError) Error() string {
	return DefaultError(e)
}

// This returns the error message without the stack trace.
func (e *DropboxBaseError) GetMessage() string {
	return e.Msg
}

// This returns the stack trace without the error message.
func (e *DropboxBaseError) GetStack() string {
	return e.Stack
}

// This returns the stack trace's context.
func (e *DropboxBaseError) GetContext() string {
	return e.Context
}

// This returns the wrapped error, if there is one.
func (e *DropboxBaseError) GetInner() error {
	return e.inner
}

func (e *DropboxBaseError) SetState(s map[string]interface{}) DropboxError {
	e.State = s
	return e
}

func (e *DropboxBaseError) GetState() map[string]interface{} {
	return e.State
}

func (e *DropboxBaseError) GetAnnotatedStates() (out []map[string]interface{}) {
	for _, err := range e.inners() {
		var s map[string]interface{}
		if dbe, ok := err.(DropboxError); ok {
			s = dbe.GetState()
			if s == nil {
				s = make(map[string]interface{})
			}
			stack := dbe.GetStack()
			if end := IndexNth(stack, "\n", 3); end != -1 {
				stack = stack[:end]
			}
			if beg := strings.LastIndex(stack, "\n"); beg != -1 {
				stack = stack[beg:]
			}
			stack = strings.TrimSpace(stack)
			s["_location"] = stack
			s["_message"] = dbe.GetMessage()
		} else {
			s = map[string]interface{}{
				"_message": err.Error(),
			}
		}

		out = append(out, s)
	}

	return
}

func (e *DropboxBaseError) HasInner(target error) (match bool) {
	for _, err := range e.inners() {
		if err == target {
			return true
		}
	}

	return false
}

func (e *DropboxBaseError) inners() (out []error) {
	var err error = e
	for {
		if err != nil {
			out = append(out, err)
		}

		if dbe, ok := err.(DropboxError); ok {
			err = dbe.GetInner()
		} else {
			return
		}
	}
}

// This returns a new DropboxBaseError initialized with the given message and
// the current stack trace.
func New(msg string) DropboxError {
	stack, context := StackTrace()
	return &DropboxBaseError{
		Msg:     msg,
		Stack:   stack,
		Context: context,
	}
}

// Same as New, but with fmt.Printf-style parameters.
func Newf(format string, args ...interface{}) DropboxError {
	stack, context := StackTrace()
	return &DropboxBaseError{
		Msg:     fmt.Sprintf(format, args...),
		Stack:   stack,
		Context: context,
	}
}

// Wraps another error in a new DropboxBaseError.
func Wrap(err error, msg string) DropboxError {
	stack, context := StackTrace()
	return &DropboxBaseError{
		Msg:     msg,
		Stack:   stack,
		Context: context,
		inner:   err,
	}
}

// Same as Wrap, but with fmt.Printf-style parameters.
func Wrapf(err error, format string, args ...interface{}) DropboxError {
	stack, context := StackTrace()
	return &DropboxBaseError{
		Msg:     fmt.Sprintf(format, args...),
		Stack:   stack,
		Context: context,
		inner:   err,
	}
}

// A default implementation of the Error method of the error interface.
func DefaultError(e DropboxError) string {
	// Find the "original" stack trace, which is probably the most helpful for
	// debugging.
	errLines := make([]string, 1)
	var origStack string
	errLines[0] = "ERROR:"
	fillErrorInfo(e, &errLines, &origStack)
	errLines = append(errLines, "")
	errLines = append(errLines, "ORIGINAL STACK TRACE:")
	errLines = append(errLines, origStack)
	return strings.Join(errLines, "\n")
}

// Fills errLines with all error messages, and origStack with the inner-most
// stack.
func fillErrorInfo(err error, errLines *[]string, origStack *string) {
	if err == nil {
		return
	}

	derr, ok := err.(DropboxError)
	if ok {
		state, err := json.Marshal(derr.GetState())
		if err != nil {
			state = []byte(err.Error())
		}
		*errLines = append(*errLines, derr.GetMessage(), string(state))
		*origStack = derr.GetStack()
		fillErrorInfo(derr.GetInner(), errLines, origStack)
	} else {
		*errLines = append(*errLines, err.Error())
	}
}

// Returns a copy of the error with the stack trace field populated and any
// other shared initialization; skips 'skip' levels of the stack trace.
//
// NOTE: This panics on any error.
func stackTrace(skip int) (current, context string) {
	// grow buf until it's large enough to store entire stack trace
	buf := make([]byte, 128)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, len(buf)*2)
	}

	// Returns the index of the first occurrence of '\n' in the buffer 'b'
	// starting with index 'start'.
	//
	// In case no occurrence of '\n' is found, it returns len(b). This
	// simplifies the logic on the calling sites.
	indexNewline := func(b []byte, start int) int {
		if start >= len(b) {
			return len(b)
		}
		searchBuf := b[start:]
		index := bytes.IndexByte(searchBuf, '\n')
		if index == -1 {
			return len(b)
		} else {
			return (start + index)
		}
	}

	// Strip initial levels of stack trace, but keep header line that
	// identifies the current goroutine.
	var strippedBuf bytes.Buffer
	index := indexNewline(buf, 0)
	if index != -1 {
		strippedBuf.Write(buf[:index])
	}

	// Skip lines.
	for i := 0; i < skip; i++ {
		index = indexNewline(buf, index+1)
		index = indexNewline(buf, index+1)
	}

	isDone := false
	startIndex := index
	lastIndex := index
	for !isDone {
		index = indexNewline(buf, index+1)
		if (index - lastIndex) <= 1 {
			isDone = true
		} else {
			lastIndex = index
		}
	}
	strippedBuf.Write(buf[startIndex:index])
	return strippedBuf.String(), string(buf[index:])
}

// This returns the current stack trace string.  NOTE: the stack creation code
// is excluded from the stack trace.
func StackTrace() (current, context string) {
	return stackTrace(3)
}
