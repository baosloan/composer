package httpx

import (
	"composer/internal/platform/errorx"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// The Bind* helpers deliberately fuse three steps that are easy to get wrong
// separately: binding, translating the validation error into something a human
// can read, and writing the failure response.
//
// The scaffold this replaced had a perfectly good message translator that no
// handler ever called, so clients received raw Go text such as
//
//	Key: 'CreateUserRequest.Username' Error:Field validation for 'Username' failed on the 'min' tag
//
// Returning a bool and writing the response internally makes that mistake
// impossible: there is no way to bind without also translating.
//
// Usage:
//
//	var in CreateIn
//	if !httpx.BindJSON(c, &in) {
//	    return // the response has already been written
//	}

// BindJSON binds and validates a JSON body. It reports whether binding
// succeeded; on failure the error response is already written.
func BindJSON(c *gin.Context, dst any) bool {
	return handle(c, c.ShouldBindJSON(dst))
}

// BindQuery binds and validates query-string parameters.
func BindQuery(c *gin.Context, dst any) bool {
	return handle(c, c.ShouldBindQuery(dst))
}

// BindURI binds and validates path parameters.
func BindURI(c *gin.Context, dst any) bool {
	return handle(c, c.ShouldBindUri(dst))
}

// BindForm binds and validates a form-encoded or multipart body.
func BindForm(c *gin.Context, dst any) bool {
	return handle(c, c.ShouldBind(dst))
}

// handle turns a binding error into a validation failure response.
func handle(c *gin.Context, err error) bool {
	if err == nil {
		return true
	}
	Error(c, bindingError(err))
	return false
}

// bindingError classifies a binding failure.
//
// Malformed JSON and failed validation are different problems and get
// different messages: one is "your request is not JSON", the other is "field X
// is wrong". Collapsing them makes debugging harder for the client.
func bindingError(err error) error {
	if errors.Is(err, io.EOF) {
		return errorx.ErrBadRequest.WithMessage("request body is empty").Wrap(err)
	}

	// Validation failures are the common case and carry per-field detail.
	if ve, ok := errors.AsType[validator.ValidationErrors](err); ok {
		return errorx.ErrValidation.WithMessage(translate(ve)).Wrap(err)
	}

	// A type mismatch names the offending field, which is far more useful than
	// a generic parse failure.
	if ute, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		field := ute.Field
		if field == "" {
			field = "body"
		}
		return errorx.ErrValidation.WithMessagef(
			"field %q expects a %s value", field, ute.Type.String()).Wrap(err)
	}

	if se, ok := errors.AsType[*json.SyntaxError](err); ok {
		return errorx.ErrBadRequest.WithMessagef(
			"request body is not valid JSON (at byte %d)", se.Offset).Wrap(err)
	}

	// A truncated body yields io.ErrUnexpectedEOF rather than a SyntaxError,
	// so it needs its own branch — otherwise the most common malformed-payload
	// case falls through to the generic "could not be parsed".
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return errorx.ErrBadRequest.
			WithMessage("request body is not valid JSON (unexpected end of input)").Wrap(err)
	}

	// http.MaxBytesReader rejects oversized bodies with a plain error string;
	// there is no sentinel to match, so match the text.
	if strings.Contains(err.Error(), "http: request body too large") {
		return errorx.ErrPayloadTooLarge.Wrap(err)
	}

	return errorx.ErrBadRequest.WithMessage("request could not be parsed").Wrap(err)
}

// translate renders validation failures as one readable sentence per field.
func translate(ve validator.ValidationErrors) string {
	parts := make([]string, 0, len(ve))
	for _, fe := range ve {
		parts = append(parts, fieldMessage(fe))
	}
	return strings.Join(parts, "; ")
}

// fieldMessage renders one failed rule.
//
// The field name comes from the json tag (see RegisterFieldNameTag), so the
// message names the field the client actually sent rather than the Go struct
// field.
func fieldMessage(fe validator.FieldError) string {
	name := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", name)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", name)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", name)
	case "uuid", "uuid4":
		return fmt.Sprintf("%s must be a valid UUID", name)
	case "min":
		return fmt.Sprintf("%s must be at least %s%s", name, fe.Param(), unit(fe))
	case "max":
		return fmt.Sprintf("%s must be at most %s%s", name, fe.Param(), unit(fe))
	case "len":
		return fmt.Sprintf("%s must be exactly %s%s", name, fe.Param(), unit(fe))
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", name, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", name, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", name, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", name, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", name,
			strings.ReplaceAll(fe.Param(), " ", ", "))
	case "eqfield":
		return fmt.Sprintf("%s must match %s", name, strings.ToLower(fe.Param()))
	case "alphanum":
		return fmt.Sprintf("%s must contain only letters and digits", name)
	case "numeric":
		return fmt.Sprintf("%s must be numeric", name)
	case "boolean":
		return fmt.Sprintf("%s must be true or false", name)
	case "strong_password":
		return fmt.Sprintf("%s must contain upper case, lower case and a digit", name)
	default:
		return fmt.Sprintf("%s is invalid", name)
	}
}

// unit appends "characters" or "items" so a numeric bound reads naturally for
// strings and slices but not for plain numbers.
func unit(fe validator.FieldError) string {
	switch fe.Kind().String() {
	case "string":
		return " characters"
	case "slice", "array", "map":
		return " items"
	default:
		return ""
	}
}
