package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EmailRegexp is a regexp you can use to validate emails
var EmailRegexp = regexp.MustCompile(`^[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}$`)

// OptionalDateRegexp is a regexp you can use to validate an optional date
var OptionalDateRegexp = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})?`)

// TimeRegexp is a regexp you can use to validate a time
var TimeRegexp = regexp.MustCompile(`\d{2}:\d{2}`)

// SlugRegexp is a regexp you can use to validate a slug
var SlugRegexp = regexp.MustCompile(`[a-z0-9_-]+`)

// ValidationFn represents a check the validator can do again some arbitrary input.
// Here fieldName's main use is for nice human readable error messages.
type ValidationFn func(values map[string]string) []string

// Validate runs the given validations against the given values
func Validate(values map[string]string, validations ...ValidationFn) []string {
	allErrors := []string{}
	for _, validation := range validations {
		errors := validation(values)
		allErrors = append(allErrors, errors...)
	}
	return allErrors
}

// ValidatePresence validates the input is a non-empty string
func ValidatePresence(field string) ValidationFn {
	return func(values map[string]string) []string {
		if len(values[field]) == 0 {
			return []string{strings.Title(field) + " is a required field"}
		}
		return []string{}
	}
}

// ValidateRegexp validates that the input matches a given regular expression
func ValidateRegexp(field string, r *regexp.Regexp) ValidationFn {
	return func(values map[string]string) []string {
		if !r.Match([]byte(values[field])) {
			return []string{strings.Title(field) + " is not in the valid format"}
		}
		return []string{}
	}
}

// ValidateLength validates the length of the value is within certain bounds.
// A min of -1 will be ignored. A max of -1 will be ignored. Min and max are
// inclusive so ValidateLength(1,3) passes [1,2,3] but fails [0,4,...]
func ValidateLength(field string, min int, max int) ValidationFn {
	return func(values map[string]string) []string {
		errors := []string{}
		if min != -1 {
			if len(values[field]) < min {
				errors = append(errors, strings.Title(field)+" is shorter than "+strconv.Itoa(min)+" characters")
			}
		}
		if max != -1 {
			if len(values[field]) > max {
				errors = append(errors, strings.Title(field)+" is longer than "+strconv.Itoa(min)+" characters")
			}
		}
		return errors
	}
}

// ValidateOneOf validates that the value is one of the provided options
func ValidateOneOf(field string, options []string) ValidationFn {
	return func(values map[string]string) []string {
		for _, o := range options {
			if values[field] == o {
				return []string{}
			}
		}
		return []string{strings.Title(field) + " is not one of " + strings.Join(options, ", ")}
	}
}

// ValidateUnique validates that no other row in the database exists for the
// given values
func ValidateUnique(field string, dbTable, dbField, currentValue string) ValidationFn {
	return func(values map[string]string) []string {
		sql := fmt.Sprintf("select count(id) as c from %s where %s = ?", dbTable, dbField)
		r := SQL(sql, values[field])
		if r[0]["c"].(float64) > 0 && values[field] != currentValue {
			return []string{strings.Title(field) + " is already taken"}
		}
		return []string{}
	}
}
