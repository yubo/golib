/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yubo/golib/api"
	"github.com/yubo/golib/runtime"
	"github.com/yubo/golib/util/validation/field"
)

// StatusError is an error intended for consumption by a REST API server; it can also be
// reconstructed by clients from a REST response. Public to allow easy type switches.
type StatusError struct {
	ErrStatus api.Status
}

// APIStatus is exposed by errors that can be converted to an api.Status object
// for finer grained details.
type APIStatus interface {
	Status() api.Status
}

var _ error = &StatusError{}

var knownReasons = map[api.StatusReason]struct{}{
	// api.StatusReasonUnknown : {}
	api.StatusReasonUnauthorized:          {},
	api.StatusReasonForbidden:             {},
	api.StatusReasonNotFound:              {},
	api.StatusReasonAlreadyExists:         {},
	api.StatusReasonConflict:              {},
	api.StatusReasonGone:                  {},
	api.StatusReasonInvalid:               {},
	api.StatusReasonServerTimeout:         {},
	api.StatusReasonTimeout:               {},
	api.StatusReasonTooManyRequests:       {},
	api.StatusReasonBadRequest:            {},
	api.StatusReasonMethodNotAllowed:      {},
	api.StatusReasonNotAcceptable:         {},
	api.StatusReasonRequestEntityTooLarge: {},
	api.StatusReasonUnsupportedMediaType:  {},
	api.StatusReasonInternalError:         {},
	api.StatusReasonExpired:               {},
	api.StatusReasonServiceUnavailable:    {},
}

// Error implements the Error interface.
func (e *StatusError) Error() string {
	return e.ErrStatus.Message
}

// Status allows access to e's status without having to know the detailed workings
// of StatusError.
func (e *StatusError) Status() api.Status {
	return e.ErrStatus
}

// DebugError reports extended info about the error to debug output.
func (e *StatusError) DebugError() (string, []interface{}) {
	if out, err := json.MarshalIndent(e.ErrStatus, "", "  "); err == nil {
		return "server response object: %s", []interface{}{string(out)}
	}
	return "server response object: %#v", []interface{}{e.ErrStatus}
}

// HasStatusCause returns true if the provided error has a details cause
// with the provided type name.
// It supports wrapped errors and returns false when the error is nil.
func HasStatusCause(err error, name api.CauseType) bool {
	_, ok := StatusCause(err, name)
	return ok
}

// StatusCause returns the named cause from the provided error if it exists and
// the error unwraps to the type APIStatus. Otherwise it returns false.
func StatusCause(err error, name api.CauseType) (api.StatusCause, bool) {
	status, ok := err.(APIStatus)
	if (ok || errors.As(err, &status)) && status.Status().Details != nil {
		for _, cause := range status.Status().Details.Causes {
			if cause.Type == name {
				return cause, true
			}
		}
	}
	return api.StatusCause{}, false
}

// UnexpectedObjectError can be returned by FromObject if it's passed a non-status object.
type UnexpectedObjectError struct {
	Object runtime.Object
}

// Error returns an error message describing 'u'.
func (u *UnexpectedObjectError) Error() string {
	return fmt.Sprintf("unexpected object: %v", u.Object)
}

// FromObject generates an StatusError from an api.Status, if that is the type of obj; otherwise,
// returns an UnexpecteObjectError.
func FromObject(obj runtime.Object) error {
	switch t := obj.(type) {
	case *api.Status:
		return &StatusError{ErrStatus: *t}
	}
	return &UnexpectedObjectError{obj}
}

// NewNotFound returns a new error which indicates that the resource of the kind and the name was not found.
func NewNotFound(name string) *StatusError {
	return &StatusError{api.Status{
		Status: api.StatusFailure,
		Code:   http.StatusNotFound,
		Reason: api.StatusReasonNotFound,
		Details: &api.StatusDetails{
			Name: name,
		},
		Message: fmt.Sprintf("%q not found", name),
	}}
}

// NewAlreadyExists returns an error indicating the item requested exists by that identifier.
func NewAlreadyExists(name string) *StatusError {
	return &StatusError{api.Status{
		Status: api.StatusFailure,
		Code:   http.StatusConflict,
		Reason: api.StatusReasonAlreadyExists,
		Details: &api.StatusDetails{
			Name: name,
		},
		Message: fmt.Sprintf(
			"%q already exists, the server was not able to generate a unique name for the object",
			name),
	}}
}

// NewUnauthorized returns an error indicating the client is not authorized to perform the requested
// action.
func NewUnauthorized(reason string) *StatusError {
	message := reason
	if len(message) == 0 {
		message = "not authorized"
	}
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusUnauthorized,
		Reason:  api.StatusReasonUnauthorized,
		Message: message,
	}}
}

// NewForbidden returns an error indicating the requested action was forbidden
func NewForbidden(name string, err error) *StatusError {
	var message string
	message = fmt.Sprintf("forbidden: %v", err)
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusForbidden,
		Reason:  api.StatusReasonForbidden,
		Details: &api.StatusDetails{Name: name},
		Message: message,
	}}
}

// NewConflict returns an error indicating the item can't be updated as provided.
func NewConflict(name string, err error) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusConflict,
		Reason:  api.StatusReasonConflict,
		Details: &api.StatusDetails{Name: name},
		Message: fmt.Sprintf("Operation cannot be fulfilled on %q: %v", name, err),
	}}
}

// NewApplyConflict returns an error including details on the requests apply conflicts
func NewApplyConflict(causes []api.StatusCause, message string) *StatusError {
	return &StatusError{ErrStatus: api.Status{
		Status: api.StatusFailure,
		Code:   http.StatusConflict,
		Reason: api.StatusReasonConflict,
		Details: &api.StatusDetails{
			// TODO: Get obj details here?
			Causes: causes,
		},
		Message: message,
	}}
}

// NewGone returns an error indicating the item no longer available at the server and no forwarding address is known.
// DEPRECATED: Please use NewResourceExpired instead.
func NewGone(message string) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusGone,
		Reason:  api.StatusReasonGone,
		Message: message,
	}}
}

// NewResourceExpired creates an error that indicates that the requested resource content has expired from
// the server (usually due to a resourceVersion that is too old).
func NewResourceExpired(message string) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusGone,
		Reason:  api.StatusReasonExpired,
		Message: message,
	}}
}

// NewInvalid returns an error indicating the item is invalid and cannot be processed.
func NewInvalid(name string, errs field.ErrorList) *StatusError {
	causes := make([]api.StatusCause, 0, len(errs))
	for i := range errs {
		err := errs[i]
		causes = append(causes, api.StatusCause{
			Type:    api.CauseType(err.Type),
			Message: err.ErrorBody(),
			Field:   err.Field,
		})
	}
	err := &StatusError{api.Status{
		Status: api.StatusFailure,
		Code:   http.StatusUnprocessableEntity,
		Reason: api.StatusReasonInvalid,
		Details: &api.StatusDetails{
			Name:   name,
			Causes: causes,
		},
	}}
	aggregatedErrs := errs.ToAggregate()
	if aggregatedErrs == nil {
		err.ErrStatus.Message = fmt.Sprintf("%q is invalid", name)
	} else {
		err.ErrStatus.Message = fmt.Sprintf("%q is invalid: %v", name, aggregatedErrs)
	}
	return err
}

// NewBadRequest creates an error that indicates that the request is invalid and can not be processed.
func NewBadRequest(reason string) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusBadRequest,
		Reason:  api.StatusReasonBadRequest,
		Message: reason,
	}}
}

// NewTooManyRequests creates an error that indicates that the client must try again later because
// the specified endpoint is not accepting requests. More specific details should be provided
// if client should know why the failure was limited.
func NewTooManyRequests(message string, retryAfterSeconds int) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusTooManyRequests,
		Reason:  api.StatusReasonTooManyRequests,
		Message: message,
		Details: &api.StatusDetails{
			RetryAfterSeconds: int32(retryAfterSeconds),
		},
	}}
}

// NewServiceUnavailable creates an error that indicates that the requested service is unavailable.
func NewServiceUnavailable(reason string) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusServiceUnavailable,
		Reason:  api.StatusReasonServiceUnavailable,
		Message: reason,
	}}
}

// NewMethodNotSupported returns an error indicating the requested action is not supported on this kind.
func NewMethodNotSupported(action string) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusMethodNotAllowed,
		Reason:  api.StatusReasonMethodNotAllowed,
		Details: &api.StatusDetails{},
		Message: fmt.Sprintf("%s is not supported on resources", action),
	}}
}

// NewServerTimeout returns an error indicating the requested action could not be completed due to a
// transient error, and the client should try again.
func NewServerTimeout(operation string, retryAfterSeconds int) *StatusError {
	return &StatusError{api.Status{
		Status: api.StatusFailure,
		Code:   http.StatusInternalServerError,
		Reason: api.StatusReasonServerTimeout,
		Details: &api.StatusDetails{
			Name:              operation,
			RetryAfterSeconds: int32(retryAfterSeconds),
		},
		Message: fmt.Sprintf("The %s operation could not be completed at this time, please try again.", operation),
	}}
}

// NewServerTimeoutForKind should not exist.  Server timeouts happen when accessing resources, the Kind is just what we
// happened to be looking at when the request failed.  This delegates to keep code sane, but we should work towards removing this.
func NewServerTimeoutForKind(operation string, retryAfterSeconds int) *StatusError {
	return NewServerTimeout(operation, retryAfterSeconds)
}

// NewInternalError returns an error indicating the item is invalid and cannot be processed.
func NewInternalError(err error) *StatusError {
	return &StatusError{api.Status{
		Status: api.StatusFailure,
		Code:   http.StatusInternalServerError,
		Reason: api.StatusReasonInternalError,
		Details: &api.StatusDetails{
			Causes: []api.StatusCause{{Message: err.Error()}},
		},
		Message: fmt.Sprintf("Internal error occurred: %v", err),
	}}
}

// NewTimeoutError returns an error indicating that a timeout occurred before the request
// could be completed.  Clients may retry, but the operation may still complete.
func NewTimeoutError(message string, retryAfterSeconds int) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusGatewayTimeout,
		Reason:  api.StatusReasonTimeout,
		Message: fmt.Sprintf("Timeout: %s", message),
		Details: &api.StatusDetails{
			RetryAfterSeconds: int32(retryAfterSeconds),
		},
	}}
}

// NewTooManyRequestsError returns an error indicating that the request was rejected because
// the server has received too many requests. Client should wait and retry. But if the request
// is perishable, then the client should not retry the request.
func NewTooManyRequestsError(message string) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusTooManyRequests,
		Reason:  api.StatusReasonTooManyRequests,
		Message: fmt.Sprintf("Too many requests: %s", message),
	}}
}

// NewRequestEntityTooLargeError returns an error indicating that the request
// entity was too large.
func NewRequestEntityTooLargeError(message string) *StatusError {
	return &StatusError{api.Status{
		Status:  api.StatusFailure,
		Code:    http.StatusRequestEntityTooLarge,
		Reason:  api.StatusReasonRequestEntityTooLarge,
		Message: fmt.Sprintf("Request entity too large: %s", message),
	}}
}

// NewGenericServerResponse returns a new error for server responses that are not in a recognizable form.
func NewGenericServerResponse(code int, verb string, name, serverMessage string, retryAfterSeconds int, isUnexpectedResponse bool) *StatusError {
	reason := api.StatusReasonUnknown
	message := fmt.Sprintf("the server responded with the status code %d but did not return more information", code)
	switch code {
	case http.StatusConflict:
		if verb == "POST" {
			reason = api.StatusReasonAlreadyExists
		} else {
			reason = api.StatusReasonConflict
		}
		message = "the server reported a conflict"
	case http.StatusNotFound:
		reason = api.StatusReasonNotFound
		message = "the server could not find the requested resource"
	case http.StatusBadRequest:
		reason = api.StatusReasonBadRequest
		message = "the server rejected our request for an unknown reason"
	case http.StatusUnauthorized:
		reason = api.StatusReasonUnauthorized
		message = "the server has asked for the client to provide credentials"
	case http.StatusForbidden:
		reason = api.StatusReasonForbidden
		// the server message has details about who is trying to perform what action.  Keep its message.
		message = serverMessage
	case http.StatusNotAcceptable:
		reason = api.StatusReasonNotAcceptable
		// the server message has details about what types are acceptable
		if len(serverMessage) == 0 || serverMessage == "unknown" {
			message = "the server was unable to respond with a content type that the client supports"
		} else {
			message = serverMessage
		}
	case http.StatusUnsupportedMediaType:
		reason = api.StatusReasonUnsupportedMediaType
		// the server message has details about what types are acceptable
		message = serverMessage
	case http.StatusMethodNotAllowed:
		reason = api.StatusReasonMethodNotAllowed
		message = "the server does not allow this method on the requested resource"
	case http.StatusUnprocessableEntity:
		reason = api.StatusReasonInvalid
		message = "the server rejected our request due to an error in our request"
	case http.StatusServiceUnavailable:
		reason = api.StatusReasonServiceUnavailable
		message = "the server is currently unable to handle the request"
	case http.StatusGatewayTimeout:
		reason = api.StatusReasonTimeout
		message = "the server was unable to return a response in the time allotted, but may still be processing the request"
	case http.StatusTooManyRequests:
		reason = api.StatusReasonTooManyRequests
		message = "the server has received too many requests and has asked us to try again later"
	default:
		if code >= 500 {
			reason = api.StatusReasonInternalError
			message = fmt.Sprintf("an error on the server (%q) has prevented the request from succeeding", serverMessage)
		}
	}
	if len(name) > 0 {
		message = fmt.Sprintf("%s (%s %s)", message, strings.ToLower(verb), name)
	}
	var causes []api.StatusCause
	if isUnexpectedResponse {
		causes = []api.StatusCause{
			{
				Type:    api.CauseTypeUnexpectedServerResponse,
				Message: serverMessage,
			},
		}
	} else {
		causes = nil
	}
	return &StatusError{api.Status{
		Status: api.StatusFailure,
		Code:   int32(code),
		Reason: reason,
		Details: &api.StatusDetails{
			Name: name,

			Causes:            causes,
			RetryAfterSeconds: int32(retryAfterSeconds),
		},
		Message: message,
	}}
}

// IsNotFound returns true if the specified error was created by NewNotFound.
// It supports wrapped errors and returns false when the error is nil.
func IsNotFound(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonNotFound {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusNotFound {
		return true
	}
	return false
}

// IsAlreadyExists determines if the err is an error which indicates that a specified resource already exists.
// It supports wrapped errors and returns false when the error is nil.
func IsAlreadyExists(err error) bool {
	return ReasonForError(err) == api.StatusReasonAlreadyExists
}

// IsConflict determines if the err is an error which indicates the provided update conflicts.
// It supports wrapped errors and returns false when the error is nil.
func IsConflict(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonConflict {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusConflict {
		return true
	}
	return false
}

// IsInvalid determines if the err is an error which indicates the provided resource is not valid.
// It supports wrapped errors and returns false when the error is nil.
func IsInvalid(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonInvalid {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusUnprocessableEntity {
		return true
	}
	return false
}

// IsGone is true if the error indicates the requested resource is no longer available.
// It supports wrapped errors and returns false when the error is nil.
func IsGone(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonGone {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusGone {
		return true
	}
	return false
}

// IsResourceExpired is true if the error indicates the resource has expired and the current action is
// no longer possible.
// It supports wrapped errors and returns false when the error is nil.
func IsResourceExpired(err error) bool {
	return ReasonForError(err) == api.StatusReasonExpired
}

// IsNotAcceptable determines if err is an error which indicates that the request failed due to an invalid Accept header
// It supports wrapped errors and returns false when the error is nil.
func IsNotAcceptable(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonNotAcceptable {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusNotAcceptable {
		return true
	}
	return false
}

// IsUnsupportedMediaType determines if err is an error which indicates that the request failed due to an invalid Content-Type header
// It supports wrapped errors and returns false when the error is nil.
func IsUnsupportedMediaType(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonUnsupportedMediaType {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusUnsupportedMediaType {
		return true
	}
	return false
}

// IsMethodNotSupported determines if the err is an error which indicates the provided action could not
// be performed because it is not supported by the server.
// It supports wrapped errors and returns false when the error is nil.
func IsMethodNotSupported(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonMethodNotAllowed {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusMethodNotAllowed {
		return true
	}
	return false
}

// IsServiceUnavailable is true if the error indicates the underlying service is no longer available.
// It supports wrapped errors and returns false when the error is nil.
func IsServiceUnavailable(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonServiceUnavailable {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusServiceUnavailable {
		return true
	}
	return false
}

// IsBadRequest determines if err is an error which indicates that the request is invalid.
// It supports wrapped errors and returns false when the error is nil.
func IsBadRequest(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonBadRequest {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusBadRequest {
		return true
	}
	return false
}

// IsUnauthorized determines if err is an error which indicates that the request is unauthorized and
// requires authentication by the user.
// It supports wrapped errors and returns false when the error is nil.
func IsUnauthorized(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonUnauthorized {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusUnauthorized {
		return true
	}
	return false
}

// IsForbidden determines if err is an error which indicates that the request is forbidden and cannot
// be completed as requested.
// It supports wrapped errors and returns false when the error is nil.
func IsForbidden(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonForbidden {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusForbidden {
		return true
	}
	return false
}

// IsTimeout determines if err is an error which indicates that request times out due to long
// processing.
// It supports wrapped errors and returns false when the error is nil.
func IsTimeout(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonTimeout {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusGatewayTimeout {
		return true
	}
	return false
}

// IsServerTimeout determines if err is an error which indicates that the request needs to be retried
// by the client.
// It supports wrapped errors and returns false when the error is nil.
func IsServerTimeout(err error) bool {
	// do not check the status code, because no https status code exists that can
	// be scoped to retryable timeouts.
	return ReasonForError(err) == api.StatusReasonServerTimeout
}

// IsInternalError determines if err is an error which indicates an internal server error.
// It supports wrapped errors and returns false when the error is nil.
func IsInternalError(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonInternalError {
		return true
	}
	if _, ok := knownReasons[reason]; !ok && code == http.StatusInternalServerError {
		return true
	}
	return false
}

// IsTooManyRequests determines if err is an error which indicates that there are too many requests
// that the server cannot handle.
// It supports wrapped errors and returns false when the error is nil.
func IsTooManyRequests(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonTooManyRequests {
		return true
	}

	// IsTooManyRequests' checking of code predates the checking of the code in
	// the other Is* functions. In order to maintain backward compatibility, this
	// does not check that the reason is unknown.
	if code == http.StatusTooManyRequests {
		return true
	}
	return false
}

// IsRequestEntityTooLargeError determines if err is an error which indicates
// the request entity is too large.
// It supports wrapped errors and returns false when the error is nil.
func IsRequestEntityTooLargeError(err error) bool {
	reason, code := reasonAndCodeForError(err)
	if reason == api.StatusReasonRequestEntityTooLarge {
		return true
	}

	// IsRequestEntityTooLargeError's checking of code predates the checking of
	// the code in the other Is* functions. In order to maintain backward
	// compatibility, this does not check that the reason is unknown.
	if code == http.StatusRequestEntityTooLarge {
		return true
	}
	return false
}

// IsUnexpectedServerError returns true if the server response was not in the expected API format,
// and may be the result of another HTTP actor.
// It supports wrapped errors and returns false when the error is nil.
func IsUnexpectedServerError(err error) bool {
	status, ok := err.(APIStatus)
	if (ok || errors.As(err, &status)) && status.Status().Details != nil {
		for _, cause := range status.Status().Details.Causes {
			if cause.Type == api.CauseTypeUnexpectedServerResponse {
				return true
			}
		}
	}
	return false
}

// IsUnexpectedObjectError determines if err is due to an unexpected object from the master.
// It supports wrapped errors and returns false when the error is nil.
func IsUnexpectedObjectError(err error) bool {
	uoe, ok := err.(*UnexpectedObjectError)
	return err != nil && (ok || errors.As(err, &uoe))
}

// SuggestsClientDelay returns true if this error suggests a client delay as well as the
// suggested seconds to wait, or false if the error does not imply a wait. It does not
// address whether the error *should* be retried, since some errors (like a 3xx) may
// request delay without retry.
// It supports wrapped errors and returns false when the error is nil.
func SuggestsClientDelay(err error) (int, bool) {
	t, ok := err.(APIStatus)
	if (ok || errors.As(err, &t)) && t.Status().Details != nil {
		switch t.Status().Reason {
		// this StatusReason explicitly requests the caller to delay the action
		case api.StatusReasonServerTimeout:
			return int(t.Status().Details.RetryAfterSeconds), true
		}
		// If the client requests that we retry after a certain number of seconds
		if t.Status().Details.RetryAfterSeconds > 0 {
			return int(t.Status().Details.RetryAfterSeconds), true
		}
	}
	return 0, false
}

// ReasonForError returns the HTTP status for a particular error.
// It supports wrapped errors and returns StatusReasonUnknown when
// the error is nil or doesn't have a status.
func ReasonForError(err error) api.StatusReason {
	if status, ok := err.(APIStatus); ok || errors.As(err, &status) {
		return status.Status().Reason
	}
	return api.StatusReasonUnknown
}

func reasonAndCodeForError(err error) (api.StatusReason, int32) {
	if status, ok := err.(APIStatus); ok || errors.As(err, &status) {
		return status.Status().Reason, status.Status().Code
	}
	return api.StatusReasonUnknown, 0
}

// ErrorReporter converts generic errors into runtime.Object errors without
// requiring the caller to take a dependency on meta/v1 (where Status lives).
// This prevents circular dependencies in core watch code.
type ErrorReporter struct {
	code   int
	verb   string
	reason string
}

// NewClientErrorReporter will respond with valid v1.Status objects that report
// unexpected server responses. Primarily used by watch to report errors when
// we attempt to decode a response from the server and it is not in the form
// we expect. Because watch is a dependency of the core api, we can't return
// meta/v1.Status in that package and so much inject this interface to convert a
// generic error as appropriate. The reason is passed as a unique status cause
// on the returned status, otherwise the generic "ClientError" is returned.
func NewClientErrorReporter(code int, verb string, reason string) *ErrorReporter {
	return &ErrorReporter{
		code:   code,
		verb:   verb,
		reason: reason,
	}
}

// AsObject returns a valid error runtime.Object (a v1.Status) for the given
// error, using the code and verb of the reporter type. The error is set to
// indicate that this was an unexpected server response.
func (r *ErrorReporter) AsObject(err error) runtime.Object {
	status := NewGenericServerResponse(r.code, r.verb, "", err.Error(), 0, true)
	if status.ErrStatus.Details == nil {
		status.ErrStatus.Details = &api.StatusDetails{}
	}
	reason := r.reason
	if len(reason) == 0 {
		reason = "ClientError"
	}
	status.ErrStatus.Details.Causes = append(status.ErrStatus.Details.Causes, api.StatusCause{
		Type:    api.CauseType(reason),
		Message: err.Error(),
	})
	return &status.ErrStatus
}
