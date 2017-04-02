// Copyright © 2017 The Things Network
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package handler

import (
	"fmt"
	"reflect"
	"time"

	"github.com/TheThingsNetwork/ttn/core/handler/functions"
	"github.com/TheThingsNetwork/ttn/utils/errors"
)

// CustomUplinkFunctions decodes, converts and validates payload using JavaScript functions
type CustomUplinkFunctions struct {
	// Decoder is a JavaScript function that accepts the payload as byte array and
	// returns an object containing the decoded values
	Decoder string
	// Converter is a JavaScript function that accepts the data as decoded by
	// Decoder and returns an object containing the converted values
	Converter string
	// Validator is a JavaScript function that validates the data is converted by
	// Converter and returns a boolean value indicating the validity of the data
	Validator string

	// Logger is the logger that will be used to store logs
	Logger functions.Logger
}

// timeOut is the maximum allowed time a payload function is allowed to run
var timeOut = 100 * time.Millisecond

// Decode decodes the payload using the Decoder function into a map
func (f *UplinkFunctions) Decode(payload []byte, port uint8) (map[string]interface{}, error) {
	if f.Decoder == "" {
		return nil, nil
	}

	env := map[string]interface{}{
		"payload": payload,
		"port":    port,
	}
	code := fmt.Sprintf(`
		%s;
		Decoder(payload.slice(0), port);
	`, f.Decoder)

	value, err := functions.RunCode("Decoder", code, env, timeOut, f.Logger)
	if err != nil {
		return nil, err
	}

	if !value.IsObject() {
		return nil, errors.NewErrInvalidArgument("Decoder", "does not return an object")
	}

	v, _ := value.Export()
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, errors.NewErrInvalidArgument("Decoder", "does not return an object")
	}
	return m, nil
}

// Convert converts the values in the specified map to a another map using the
// Converter function. If the Converter function is not set, this function
// returns the data as-is
func (f *UplinkFunctions) Convert(fields map[string]interface{}, port uint8) (map[string]interface{}, error) {
	if f.Converter == "" {
		return fields, nil
	}

	env := map[string]interface{}{
		"fields": fields,
		"port":   port,
	}

	code := fmt.Sprintf(`
		%s;
		Converter(fields, port)
	`, f.Converter)

	value, err := functions.RunCode("Converter", code, env, timeOut, f.Logger)
	if err != nil {
		return nil, err
	}

	if !value.IsObject() {
		return nil, errors.NewErrInvalidArgument("Converter", "does not return an object")
	}

	v, _ := value.Export()
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, errors.NewErrInvalidArgument("Converter", "does not return an object")
	}

	return m, nil
}

// Validate validates the values in the specified map using the Validator
// function. If the Validator function is not set, this function returns true
func (f *UplinkFunctions) Validate(fields map[string]interface{}, port uint8) (bool, error) {
	if f.Validator == "" {
		return true, nil
	}

	env := map[string]interface{}{
		"fields": fields,
		"port":   port,
	}
	code := fmt.Sprintf(`
		%s;
		Validator(fields, port)
	`, f.Validator)

	value, err := functions.RunCode("Validator", code, env, timeOut, f.Logger)
	if err != nil {
		return false, err
	}

	if !value.IsBoolean() {
		return false, errors.NewErrInvalidArgument("Validator", "does not return a boolean")
	}

	return value.ToBoolean()
}

// Decode decodes the specified payload, converts it and tests the validity
func (f *UplinkFunctions) Decode(payload []byte, port uint8) (map[string]interface{}, bool, error) {
	decoded, err := f.Decode(payload, port)
	if err != nil {
		return nil, false, err
	}

	converted, err := f.Convert(decoded, port)
	if err != nil {
		return nil, false, err
	}

	valid, err := f.Validate(converted, port)
	return converted, valid, err
}

// CustomDownlinkFunctions encodes payload using JavaScript functions
type CustomDownlinkFunctions struct {
	// Encoder is a JavaScript function that accepts the payload as JSON and
	// returns an array of bytes
	Encoder string

	// Logger is the logger that will be used to store logs
	Logger functions.Logger
}

// Encode encodes the map into a byte slice using the encoder payload function
// If no encoder function is set, this function returns an array.
func (f *DownlinkFunctions) Encode(payload map[string]interface{}, port uint8) ([]byte, error) {
	if f.Encoder == "" {
		return nil, errors.NewErrInvalidArgument("Downlink Payload", "fields supplied, but no Encoder function set")
	}

	env := map[string]interface{}{
		"payload": payload,
		"port":    port,
	}
	code := fmt.Sprintf(`
		%s;
		Encoder(payload, port)
	`, f.Encoder)

	value, err := functions.RunCode("Encoder", code, env, timeOut, f.Logger)
	if err != nil {
		return nil, err
	}

	if !value.IsObject() {
		return nil, errors.NewErrInvalidArgument("Encoder", "does not return an object")
	}

	v, err := value.Export()
	if err != nil {
		return nil, err
	}

	if reflect.TypeOf(v).Kind() != reflect.Slice {
		return nil, errors.NewErrInvalidArgument("Encoder", "does not return an Array")
	}

	s := reflect.ValueOf(v)
	l := s.Len()

	res := make([]byte, l)

	var n int64
	for i := 0; i < l; i++ {
		el := s.Index(i).Interface()

		// type switch does not have fallthrough so we need
		// to check every element individually
		switch t := el.(type) {
		case byte:
			n = int64(t)
		case int:
			n = int64(t)
		case int8:
			n = int64(t)
		case int16:
			n = int64(t)
		case uint16:
			n = int64(t)
		case int32:
			n = int64(t)
		case uint32:
			n = int64(t)
		case int64:
			n = int64(t)
		case uint64:
			n = int64(t)
		case float32:
			n = int64(t)
			if float32(n) != t {
				return nil, errors.NewErrInvalidArgument("Encoder", "should return an Array of integer numbers")
			}
		case float64:
			n = int64(t)
			if float64(n) != t {
				return nil, errors.NewErrInvalidArgument("Encoder", "should return an Array of integer numbers")
			}
		default:
			return nil, errors.NewErrInvalidArgument("Encoder", "should return an Array of integer numbers")
		}

		if n < 0 || n > 255 {
			return nil, errors.NewErrInvalidArgument("Encoder Output", "Numbers in Array should be between 0 and 255")
		}

		res[i] = byte(n)
	}

	return res, nil
}

// Encode encodes the specified field, converts it into a valid payload
func (f *DownlinkFunctions) Encode(payload map[string]interface{}, port uint8) ([]byte, bool, error) {
	encoded, err := f.Encode(payload, port)
	if err != nil {
		return nil, false, err
	}

	return encoded, true, nil
}
