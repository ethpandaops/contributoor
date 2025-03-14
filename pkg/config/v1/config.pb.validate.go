// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: config/v1/config.proto

package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
	_ = sort.Sort
)

// Validate checks the field values on Config with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Config) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Config with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in ConfigMultiError, or nil if none found.
func (m *Config) ValidateAll() error {
	return m.validate(true)
}

func (m *Config) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for LogLevel

	// no validation rules for Version

	// no validation rules for ContributoorDirectory

	// no validation rules for RunMethod

	// no validation rules for NetworkName

	// no validation rules for BeaconNodeAddress

	// no validation rules for MetricsAddress

	// no validation rules for PprofAddress

	if all {
		switch v := interface{}(m.GetOutputServer()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, ConfigValidationError{
					field:  "OutputServer",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, ConfigValidationError{
					field:  "OutputServer",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetOutputServer()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return ConfigValidationError{
				field:  "OutputServer",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	// no validation rules for DockerNetwork

	// no validation rules for HealthCheckAddress

	if len(errors) > 0 {
		return ConfigMultiError(errors)
	}

	return nil
}

// ConfigMultiError is an error wrapping multiple validation errors returned by
// Config.ValidateAll() if the designated constraints aren't met.
type ConfigMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m ConfigMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m ConfigMultiError) AllErrors() []error { return m }

// ConfigValidationError is the validation error returned by Config.Validate if
// the designated constraints aren't met.
type ConfigValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e ConfigValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e ConfigValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e ConfigValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e ConfigValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e ConfigValidationError) ErrorName() string { return "ConfigValidationError" }

// Error satisfies the builtin error interface
func (e ConfigValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sConfig.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = ConfigValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = ConfigValidationError{}

// Validate checks the field values on OutputServer with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *OutputServer) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on OutputServer with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in OutputServerMultiError, or
// nil if none found.
func (m *OutputServer) ValidateAll() error {
	return m.validate(true)
}

func (m *OutputServer) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Address

	// no validation rules for Credentials

	// no validation rules for Tls

	if len(errors) > 0 {
		return OutputServerMultiError(errors)
	}

	return nil
}

// OutputServerMultiError is an error wrapping multiple validation errors
// returned by OutputServer.ValidateAll() if the designated constraints aren't met.
type OutputServerMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m OutputServerMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m OutputServerMultiError) AllErrors() []error { return m }

// OutputServerValidationError is the validation error returned by
// OutputServer.Validate if the designated constraints aren't met.
type OutputServerValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e OutputServerValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e OutputServerValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e OutputServerValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e OutputServerValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e OutputServerValidationError) ErrorName() string { return "OutputServerValidationError" }

// Error satisfies the builtin error interface
func (e OutputServerValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sOutputServer.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = OutputServerValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = OutputServerValidationError{}
