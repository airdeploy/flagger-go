package core

import (
	"time"

	"github.com/airdeploy/flagger-go/v3/log"
)

const (
	filterTypeBool   = "BOOLEAN"
	filterTypeNumber = "NUMBER"
	filterTypeString = "STRING"
	filterTypeDate   = "DATE"
)

// This function matches filters with entity
// It returns true if none of the filters returns false
func matchByFilters(filters []*FlagFilter, attributes Attributes) bool {
	if len(filters) == 0 {
		return true
	}

	if attributes == nil {
		return false
	}

	// make lower case all attributes keys
	attributes = escapeAttributes(attributes)

	// preparing the filters for matching
	for _, filter := range filters {
		filter.escape()
	}

	for _, filter := range filters {
		attr, ok := attributes[filter.AttributeName]

		if !ok {
			if filter.Operator == isNot {
				return true // attribute is not present so return true
			}
			if filter.Operator == notIn {
				return true // attribute is not present so return true
			}
			return false // attribute is expected so false
		}

		// return by false from assert function or mismatch by types
		switch filterValue := filter.Value.(type) {
		case string:
			attrStr, ok := attr.(string)
			if !ok {
				log.Warnf("Type mismatch for attribute \"%+v\", expected string, but got: %T", filter.AttributeName, attr)
				return false
			}
			if /* NOT */ !assertForString(filter.Operator, filterValue, attrStr, filter.AttributeName) {
				return false
			}

		case time.Time:
			attrStr, ok := attr.(string)
			if !ok {
				log.Warnf("Type mismatch for attribute \"%+v\", expected Date.string(), but got: %T", filter.AttributeName, attr)
				return false
			}

			attrDate, err := time.Parse(time.RFC3339, attrStr)
			if err != nil {
				log.Warnf("Cannot parse value \"%+v\" for attribute \"%+v\" as RFC3339(\"%+v\")", attrStr, filter.AttributeName, time.RFC3339)
				return false
			}

			if /* NOT */ !assertForDate(filter.Operator, filterValue, attrDate, filter.AttributeName) {
				return false
			}

		// filterValue type will never be int, because json number is parsed as float64
		case int:
			return false

		// encoding.json lib parse any number as float64
		case float64:
			switch v := attr.(type) {
			// escapeAttributes converts int to float64
			case float64:
				if /* NOT */ !assertForFloat(filter.Operator, filterValue, v, filter.AttributeName) {
					return false
				}
			default:
				log.Warnf("Type mismatch for attribute \"%+v\", expected float64, but got: %T", filter.AttributeName, attr)
				return false
			}

		case bool:
			attrBool, ok := attr.(bool)
			if !ok {
				log.Warnf("Type mismatch for attribute \"%+v\", expected bool, but got: %T", filter.AttributeName, attr)
				return false
			}
			if /* NOT */ !assertForBool(filter.Operator, filterValue, attrBool, filter.AttributeName) {
				return false
			}

		case []string:
			attrStr, ok := attr.(string)
			if !ok {
				log.Warnf("Type mismatch for attribute \"%+v\", expected string, but got: %T", filter.AttributeName, attr)
				return false
			}
			if /* NOT */ !assertForStringArr(filter.Operator, filterValue, attrStr, filter.AttributeName) {
				return false
			}

		// filterValue type will never be int, because json number is parsed as float64
		case []int:
			return false

		case []float64:
			attrFloat, ok := attr.(float64)
			if !ok {
				log.Warnf("Type mismatch for attribute \"%+v\", expected float64, but got: %T", filter.AttributeName, attr)
				return false
			}
			if /* NOT */ !assertForFloatArr(filter.Operator, filterValue, attrFloat, filter.AttributeName) {
				return false
			}

		case []bool:
			attrBool, ok := attr.(bool)
			if !ok {
				log.Warnf("Type mismatch for attribute \"%+v\", expected bool, but got: %T", filter.AttributeName, attr)
				return false
			}
			if /* NOT */ !asertForBoolArr(filter.Operator, filterValue, attrBool, filter.AttributeName) {
				return false
			}

		case []time.Time:
			attrStr, ok := attr.(string)
			if !ok {
				log.Warnf("Type mismatch for attribute \"%+v\", expected Date.string(), but got: %T", filter.AttributeName, attr)
				return false
			}

			attrDate, err := time.Parse(time.RFC3339, attrStr)
			if err != nil {
				log.Warnf("Cannot parse value \"%+v\" for attribute \"%+v\" as RFC3339(\"%+v\")", attrStr, filter.AttributeName, time.RFC3339)
				return false
			}

			if /* NOT */ !assertForDateArr(filter.Operator, filterValue, attrDate, filter.AttributeName) {
				return false
			}

		default:
			log.Warnf("Filter value type mismatch for attribute \"%+v\", expected: bool, string, float64, date or array, but got: %T", filter.AttributeName, filterValue)
			return false
		}
	}

	// we have filters and all was matched
	return true
}

func assertForString(op Operator, filterValue, attributeValue, attributeName string) bool {
	switch op {
	case is, in:
		return filterValue == attributeValue
	case isNot, notIn:
		return filterValue != attributeValue
	default:
		log.Warnf("Cannot use operator \"%+v\" for string, attribute: \"%+v\", value: \"%+v\", filter: \"%+v\"",
			op, attributeName, attributeValue, filterValue)
		return false
	}
}

func assertForStringArr(op Operator, filterValue []string, attributeValue, attributeName string) bool {
	switch op {
	case in:
		for _, v := range filterValue {
			if attributeValue == v {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range filterValue {
			if attributeValue == v {
				return false
			}
		}
		return true
	default:
		log.Warnf("Cannot use operator \"%+v\" for []string, attribute: \"%+v\", value: \"%+v\", filter: \"%+v\"",
			op, attributeName, attributeValue, filterValue)
		return false
	}
}

func assertForDate(op Operator, filterValue, attributeValue time.Time, attributeName string) bool {
	switch op {
	case is:
		return filterValue.Equal(attributeValue)
	case isNot:
		return !filterValue.Equal(attributeValue)
	case lt:
		return attributeValue.Before(filterValue)
	case lte:
		return attributeValue.Before(filterValue) || attributeValue.Equal(filterValue)
	case gt:
		return attributeValue.After(filterValue)
	case gte:
		return attributeValue.After(filterValue) || attributeValue.Equal(filterValue)
	default:
		log.Warnf("Cannot use operator \"%+v\" for date, attribute: \"%+v\", value: \"%+v\", filter: \"%+v\"",
			op, attributeName, attributeValue, filterValue)
		return false
	}
}

func assertForDateArr(op Operator, filterValue []time.Time, attributeValue time.Time, attributeName string) bool {
	switch op {
	case in:
		for _, v := range filterValue {
			if attributeValue.Equal(v) {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range filterValue {
			if attributeValue.Equal(v) {
				return false
			}
		}
		return true
	default:
		log.Warnf("Cannot use operator \"%+v\" for []date, attribute: \"%+v\", value: \"%+v\", filter: \"%+v\"",
			op, attributeName, attributeValue, filterValue)
		return false
	}
}

func assertForFloat(op Operator, filterValue, attributeValue float64, attributeName string) bool {
	switch op {
	case is:
		return filterValue == attributeValue
	case isNot:
		return filterValue != attributeValue
	case lt:
		return attributeValue < filterValue
	case lte:
		return attributeValue <= filterValue
	case gt:
		return attributeValue > filterValue
	case gte:
		return attributeValue >= filterValue
	default:
		log.Warnf("Cannot use operator \"%+v\" for number, attribute: \"%+v\", value: \"%+v\", filter: \"%+v\"",
			op, attributeName, attributeValue, filterValue)
		return false
	}
}

func assertForFloatArr(op Operator, filterValue []float64, attributeValue float64, attributeName string) bool {
	switch op {
	case in:
		for _, v := range filterValue {
			if attributeValue == v {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range filterValue {
			if attributeValue == v {
				return false
			}
		}
		return true
	default:
		log.Warnf("Cannot use operator \"%+v\" for []number, attribute: \"%+v\", value: \"%+v\", filter: \"%+v\"",
			op, attributeName, attributeValue, filterValue)
		return false
	}
}

func assertForBool(op Operator, filterValue, attributeValue bool, attributeName string) bool {
	switch op {
	case is:
		return filterValue == attributeValue
	case isNot:
		return filterValue != attributeValue
	default:
		log.Warnf("Cannot use operator \"%+v\" for boolean, attribute: \"%+v\", value: \"%+v\", filter: \"%+v\"",
			op, attributeName, attributeValue, filterValue)
		return false
	}
}

func asertForBoolArr(op Operator, filterValue []bool, attributeValue bool, attributeName string) bool {
	switch op {
	case in:
		for _, v := range filterValue {
			if attributeValue == v {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range filterValue {
			if attributeValue == v {
				return false
			}
		}
		return true
	default:
		log.Warnf("Cannot use operator \"%+v\" for []boolean, attribute: \"%+v\", value: \"%+v\", filter: \"%+v\"",
			op, attributeName, attributeValue, filterValue)
		return false
	}
}
