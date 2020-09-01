package core

import (
	"time"

	"github.com/airdeploy/flagger-go/v3/log"
	"github.com/pkg/errors"
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

	// make lower all filters attribute names
	for _, filter := range filters {
		filter.escape()
	}

	for _, filter := range filters {
		attr, ok := attributes[filter.AttributeName]
		if !ok && filter.Operator != isNot && filter.Operator != notIn {
			return false
		}

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
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as string, but got: %T", filter.AttributeName, attr))
				return false
			}
			if /* NOT */ !assertForString(filter.Operator, filterValue, attrStr) {
				return false
			}

		case time.Time:
			attrStr, ok := attr.(string)
			if !ok {
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as Date.string(), but got: %T", filter.AttributeName, attr))
				return false
			}

			attrDate, err := time.Parse(time.RFC3339, attrStr)
			if err != nil {
				log.Warnf("%+v", errors.Wrapf(err, "parse %+v %s", attrStr, err.Error()))
				return false
			}

			if /* NOT */ !assertForDate(filter.Operator, filterValue, attrDate) {
				return false
			}

		// filterValue type will never be int, because json number is parsed as float64
		case int:
			return false

		// encoding.json lib parse any number as float64
		case float64:
			switch v := attr.(type) {
			case float64:
				if /* NOT */ !assertForFloat(filter.Operator, filterValue, v) {
					return false
				}
			case int:
				if /* NOT */ !assertForFloat(filter.Operator, filterValue, float64(v)) {
					return false
				}
			default:
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as float64, but got: %T", filter.AttributeName, attr))
				return false
			}

		case bool:
			attrBool, ok := attr.(bool)
			if !ok {
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as bool, but got: %T", filter.AttributeName, attr))
				return false
			}
			if /* NOT */ !assertForBool(filter.Operator, filterValue, attrBool) {
				return false
			}

		case []string:
			attrStr, ok := attr.(string)
			if !ok {
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as string, but got: %T", filter.AttributeName, attr))
				return false
			}
			if /* NOT */ !assertForStringArr(filter.Operator, filterValue, attrStr) {
				return false
			}

		case []int:
			attrInt, ok := attr.(int)
			if !ok {
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as int, but got: %T", filter.AttributeName, attr))
				return false
			}
			if /* NOT */ !assertForIntArr(filter.Operator, filterValue, attrInt) {
				return false
			}

		case []float64:
			attrFloat, ok := attr.(float64)
			if !ok {
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as float64, but got: %T", filter.AttributeName, attr))
				return false
			}
			if /* NOT */ !assertForFloatArr(filter.Operator, filterValue, attrFloat) {
				return false
			}

		case []bool:
			attrBool, ok := attr.(bool)
			if !ok {
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as bool, but got: %T", filter.AttributeName, attr))
				return false
			}
			if /* NOT */ !asertForBoolArr(filter.Operator, filterValue, attrBool) {
				return false
			}

		case []time.Time:
			attrStr, ok := attr.(string)
			if !ok {
				log.Warnf("%+v", errors.Errorf("expect entity.Value[%v] as Date.string(), but got: %T", filter.AttributeName, attr))
				return false
			}

			attrDate, err := time.Parse(time.RFC3339, attrStr)
			if err != nil {
				log.Warnf("%+v", errors.Wrapf(err, "parse %+v %s", attrStr, err.Error()))
				return false
			}

			if /* NOT */ !assertForDateArr(filter.Operator, filterValue, attrDate) {
				return false
			}

		default:
			log.Warnf("%+v", errors.Errorf("unexpect entity.Value[%v] type: %T", filter.AttributeName, attr))
			return false
		}
	}

	// we have filters and all was matched
	return true
}

func assertForString(op Operator, fv, av string) bool {
	switch op {
	case is, in:
		return fv == av
	case isNot, notIn:
		return fv != av
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}

func assertForStringArr(op Operator, fv []string, av string) bool {
	switch op {
	case in:
		for _, v := range fv {
			if av == v {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range fv {
			if av == v {
				return false
			}
		}
		return true
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}

func assertForDate(op Operator, fv, av time.Time) bool {
	switch op {
	case is:
		return /* fv == av */ fv.Equal(av)
	case isNot:
		return /* fv != av */ !fv.Equal(av)
	case lt:
		return /* av < fv */ av.Before(fv)
	case lte:
		return /* av <= fv */ av.Before(fv) || av.Equal(fv)
	case gt:
		return /* av > fv */ av.After(fv)
	case gte:
		return /* av >= fv */ av.After(fv) || av.Equal(fv)
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}

func assertForDateArr(op Operator, fv []time.Time, av time.Time) bool {
	switch op {
	case in:
		for _, v := range fv {
			if av.Equal(v) {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range fv {
			if av.Equal(v) {
				return false
			}
		}
		return true
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}

func assertForIntArr(op Operator, fv []int, av int) bool {
	switch op {
	case in:
		for _, v := range fv {
			if av == v {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range fv {
			if av == v {
				return false
			}
		}
		return true
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}

func assertForFloat(op Operator, fv, av float64) bool {
	switch op {
	case is:
		return fv == av
	case isNot:
		return fv != av
	case lt:
		return av < fv
	case lte:
		return av <= fv
	case gt:
		return av > fv
	case gte:
		return av >= fv
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}

func assertForFloatArr(op Operator, fv []float64, av float64) bool {
	switch op {
	case in:
		for _, v := range fv {
			if av == v {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range fv {
			if av == v {
				return false
			}
		}
		return true
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}

func assertForBool(op Operator, fv, av bool) bool {
	switch op {
	case is:
		return fv == av
	case isNot:
		return fv != av
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}

func asertForBoolArr(op Operator, fv []bool, av bool) bool {
	switch op {
	case in:
		for _, v := range fv {
			if av == v {
				return true
			}
		}
		return false
	case notIn:
		for _, v := range fv {
			if av == v {
				return false
			}
		}
		return true
	default:
		log.Warnf("%+v", errors.Errorf("unsupported operator: %+v for %+v %+v", op, fv, av))
		return false
	}
}
