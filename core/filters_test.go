package core

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_matchByFilters(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		attr := Attributes{}
		filters := []*FlagFilter{}
		assert.True(t, matchByFilters(nil, nil))
		assert.True(t, matchByFilters(nil, attr))
		assert.True(t, matchByFilters(filters, nil))
		assert.True(t, matchByFilters(filters, attr))

		filters = []*FlagFilter{{}}
		assert.False(t, matchByFilters(filters, nil))
		assert.False(t, matchByFilters(filters, attr))
	})

	t.Run("simple", func(t *testing.T) {

		t.Run("pos1", func(t *testing.T) {
			country := randCountry()
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      is,
						Value:         country,
						FilterType:    filterTypeString,
					},
				},
				randAttributes(
					add("country", country),
				)))
		})

		t.Run("neg1", func(t *testing.T) {
			country := randCountry()
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      is,
						Value:         country,
						FilterType:    filterTypeString,
					},
				},
				randAttributes(
					add("country", randCountryNEq(country)),
				)))
		})
	})

	t.Run("no such attribute", func(t *testing.T) {
		assert.False(t, matchByFilters(
			[]*FlagFilter{
				{
					AttributeName: "country",
					Operator:      is,
					Value:         randCountry(),
					FilterType:    filterTypeString,
				},
			},
			randAttributes(
				del("country"),
			)))
	})

	t.Run("broken filters", func(t *testing.T) {
		assert.False(t, matchByFilters(
			[]*FlagFilter{
				{
					AttributeName: "country",
					Operator:      in,
					Value: []interface{}{
						randInt(),
						randCountry(),
						randBool()},
					FilterType: filterTypeString,
				},
			},
			randAttributes()))
	})

	t.Run("type mismatch", func(t *testing.T) {
		t.Run("pos1", func(t *testing.T) {
			age := randInt()
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "age",
						Operator:      is,
						Value:         age,
						FilterType:    filterTypeNumber,
					},
				},
				randAttributes(
					add("age", strconv.Itoa(age)),
				)))
		})

		t.Run("pos2", func(t *testing.T) {
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      is,
						Value:         randInt(),
						FilterType:    filterTypeNumber,
					},
				},
				randAttributes(
					add("probability", randFloat()),
				)))
		})
	})

	t.Run("is", func(t *testing.T) {
		t.Run("string", func(t *testing.T) {

			t.Run("pos1", func(t *testing.T) {
				country := randCountry()
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "country",
							Operator:      is,
							Value:         country,
							FilterType:    filterTypeString,
						},
					},
					randAttributes(
						add("country", country),
					)))
			})

			t.Run("neg1", func(t *testing.T) {
				country := randCountry()
				assert.False(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "country",
							Operator:      is,
							Value:         country,
							FilterType:    filterTypeString,
						},
					},
					randAttributes(
						add("country", randCountryNEq(country)),
					)))
			})
		})

		t.Run("float", func(t *testing.T) {

			t.Run("pos1", func(t *testing.T) {
				probability := randFloat()
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "probability",
							Operator:      is,
							Value:         probability,
							FilterType:    filterTypeNumber,
						},
					},
					randAttributes(
						add("probability", probability),
					)))
			})

			t.Run("filter float, attribute int", func(t *testing.T) {
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "friends",
							Operator:      is,
							Value:         100.0,
							FilterType:    filterTypeNumber,
						},
					},
					randAttributes(
						add("friends", 100),
					)))

			})

			t.Run("neg1", func(t *testing.T) {
				probability := randFloat()
				assert.False(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "probability",
							Operator:      is,
							Value:         probability,
							FilterType:    filterTypeNumber,
						},
					},
					randAttributes(
						add("probability", randFloatNEq(probability)),
					)))
			})
		})

		t.Run("bool", func(t *testing.T) {

			t.Run("pos1", func(t *testing.T) {
				admin := randBool()
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "admin",
							Operator:      is,
							Value:         admin,
							FilterType:    filterTypeBool,
						},
					},
					randAttributes(
						add("admin", admin),
					)))
			})

			t.Run("neg1", func(t *testing.T) {
				admin := randBool()
				assert.False(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "admin",
							Operator:      is,
							Value:         admin,
							FilterType:    filterTypeBool,
						},
					},
					randAttributes(
						add("admin", randBoolNEq(admin)),
					)))
			})
		})

		t.Run("date", func(t *testing.T) {

			createdAt := "2016-03-16T05:44:23Z"
			t.Run("positive test", func(t *testing.T) {
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "createdAt",
							Operator:      is,
							Value:         createdAt,
							FilterType:    filterTypeDate,
						},
					},
					randAttributes(
						add("createdAt", createdAt),
					)))
			})
			t.Run("negative tests", func(t *testing.T) {
				t.Run("client's value is a number", func(t *testing.T) {
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      is,
								Value:         createdAt,
								FilterType:    filterTypeDate,
							},
						},
						randAttributes(
							add("createdAt", 711185707000),
						)))
				})
				t.Run("client's value is a number in string", func(t *testing.T) {
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      is,
								Value:         createdAt,
								FilterType:    filterTypeDate,
							},
						},
						randAttributes(
							add("createdAt", "711185707000"),
						)))
				})
				t.Run("client's value is in the wrong format, RFC 2822", func(t *testing.T) {
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      is,
								Value:         createdAt,
								FilterType:    filterTypeDate,
							},
						},
						randAttributes(
							add("createdAt", "Fri, 26 Dec 7000 12:12:06 -0200"),
						)))
				})
				t.Run("server's value is a number", func(t *testing.T) {
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      is,
								Value:         711185707000,
								FilterType:    filterTypeDate,
							},
						},
						randAttributes(
							add("createdAt", "2016-03-16T05:44:23Z"),
						)))
				})
				t.Run("client's value is a string", func(t *testing.T) {
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      is,
								Value:         createdAt,
								FilterType:    filterTypeDate,
							},
						},
						randAttributes(
							add("createdAt", "fdsfdsfsd"),
						)))
				})
				t.Run("client's value is an array", func(t *testing.T) {
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      is,
								Value:         createdAt,
								FilterType:    filterTypeDate,
							},
						},
						randAttributes(
							add("createdAt", []string{"2016-03-16T05:44:23Z"}),
						)))
				})
				t.Run("client's value is a boolean", func(t *testing.T) {
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      is,
								Value:         createdAt,
								FilterType:    filterTypeDate,
							},
						},
						randAttributes(
							add("createdAt", true),
						)))
				})
				t.Run("values don't match", func(t *testing.T) {
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      is,
								Value:         createdAt,
								FilterType:    filterTypeDate,
							},
						},
						randAttributes(
							add("createdAt", "2019-09-16T05:44:23Z"),
						)))
				})
			})

			t.Run("neg1", func(t *testing.T) {
				createdAt := randTS()
				assert.False(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "createdAt",
							Operator:      is,
							Value:         createdAt,
							FilterType:    filterTypeDate,
						},
					},
					randAttributes(
						add("createdAt", randTSNEq(createdAt).Format(time.RFC3339)),
					)))
			})
		})
	})

	t.Run("is_not", func(t *testing.T) {
		t.Run("string", func(t *testing.T) {

			t.Run("pos1", func(t *testing.T) {
				country := randCountry()
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "country",
							Operator:      isNot,
							Value:         country,
							FilterType:    filterTypeString,
						},
					},
					Attributes{
						"country":     randCountryNEq(country),
						"age":         randInt(),
						"admin":       randBool(),
						"probability": randFloat(),
					}))
			})

			t.Run("pos1", func(t *testing.T) {
				// have no attribute country
				country := randCountry()
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "country",
							Operator:      isNot,
							Value:         country,
							FilterType:    filterTypeString,
						},
					},
					Attributes{
						"age":         randInt(),
						"admin":       randBool(),
						"probability": randFloat(),
					}))
			})

			t.Run("neg1", func(t *testing.T) {
				country := randCountry()
				assert.False(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "country",
							Operator:      isNot,
							Value:         country,
							FilterType:    filterTypeString,
						},
					},
					Attributes{
						"country":     country,
						"age":         randInt(),
						"admin":       randBool(),
						"probability": randFloat(),
					}))
			})
		})

		t.Run("float", func(t *testing.T) {

			t.Run("pos1", func(t *testing.T) {
				probability := randFloat()
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "probability",
							Operator:      isNot,
							Value:         probability,
							FilterType:    filterTypeNumber,
						},
					},
					Attributes{
						"country":     randCountry(),
						"age":         randInt(),
						"admin":       randBool(),
						"probability": randFloatNEq(probability),
					}))
			})

			t.Run("pos2", func(t *testing.T) {
				// have no attribute probability
				probability := randFloat()
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "probability",
							Operator:      isNot,
							Value:         probability,
							FilterType:    filterTypeNumber,
						},
					},
					Attributes{
						"country": randCountry(),
						"age":     randInt(),
						"admin":   randBool(),
					}))
			})

			t.Run("neg1", func(t *testing.T) {
				probability := randFloat()
				assert.False(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "probability",
							Operator:      isNot,
							Value:         probability,
							FilterType:    filterTypeNumber,
						},
					},
					Attributes{
						"country":     randCountry(),
						"age":         randInt(),
						"admin":       randBool(),
						"probability": probability,
					}))
			})
		})

		t.Run("bool", func(t *testing.T) {

			t.Run("pos1", func(t *testing.T) {
				admin := randBool()
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "admin",
							Operator:      isNot,
							Value:         admin,
							FilterType:    filterTypeBool,
						},
					},
					Attributes{
						"country":     randCountry(),
						"age":         randInt(),
						"admin":       randBoolNEq(admin),
						"probability": randFloat(),
					}))
			})

			// positive, no attribute admin
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "admin",
						Operator:      isNot,
						Value:         true,
						FilterType:    filterTypeBool,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         21,
					"probability": 0.5,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "admin",
						Operator:      isNot,
						Value:         true,
						FilterType:    filterTypeBool,
					},
				},
				Attributes{
					"country":     "FR",
					"age":         36,
					"admin":       true,
					"probability": 0.9,
				}))
		})

		t.Run("date", func(t *testing.T) {
			now := time.Unix(time.Now().Unix(), 0)

			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      isNot,
						Value:         now,
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         21,
					"createdAt":   now.Add(5 * time.Hour).Format(time.RFC3339),
					"probability": 0.5,
				}))

			// positive, no attribute createdAt
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      isNot,
						Value:         now,
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         21,
					"probability": 0.5,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      isNot,
						Value:         now,
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"country":     "FR",
					"age":         36,
					"createdAt":   now.Format(time.RFC3339),
					"probability": 0.9,
				}))
		})
	})

	t.Run("lt", func(t *testing.T) {

		t.Run("float", func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      lt,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         20,
					"probability": 0.4,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      lt,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         25,
					"probability": 0.5,
				}))
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      lt,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         25,
					"probability": 0.9,
				}))
		})

		t.Run("date", func(t *testing.T) {
			now := time.Unix(time.Now().Unix(), 0)

			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      lt,
						Value:         now,
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"age":       20,
					"createdAt": now.Add(-3 * time.Hour).Format(time.RFC3339),
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      lt,
						Value:         now,
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"age":       25,
					"createdAt": now.Format(time.RFC3339),
				}))
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      lt,
						Value:         now,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":       25,
					"createdAt": now.Add(5 * time.Hour).Format(time.RFC3339),
				}))
		})
	})

	t.Run("lte", func(t *testing.T) {

		t.Run("float", func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      lte,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         20,
					"probability": 0.4,
				}))

			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      lte,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         20,
					"probability": 0.5,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      lte,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         25,
					"probability": 0.9,
				}))
		})

		t.Run("date", func(t *testing.T) {
			now := time.Unix(time.Now().Unix(), 0)

			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      lt,
						Value:         now,
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"age":       20,
					"createdAt": now.Add(-3 * time.Hour).Format(time.RFC3339),
				}))
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      lt,
						Value:         now,
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"age":       25,
					"createdAt": now.Format(time.RFC3339),
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      lt,
						Value:         now,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":       25,
					"createdAt": now.Add(5 * time.Hour).Format(time.RFC3339),
				}))
		})
	})

	t.Run("gt", func(t *testing.T) {

		t.Run("float", func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      gt,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         20,
					"probability": 0.7,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      gt,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         25,
					"probability": 0.1,
				}))
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      gt,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         25,
					"probability": 0.5,
				}))
		})

		t.Run("date", func(t *testing.T) {
			now := time.Unix(time.Now().Unix(), 0)

			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      gt,
						Value:         now,
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"age":       20,
					"createdAt": now.Add(3 * time.Hour).Format(time.RFC3339),
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      gt,
						Value:         now,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":       20,
					"createdAt": now.Format(time.RFC3339),
				}))
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      gt,
						Value:         now,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":       20,
					"createdAt": now.Add(-7 * time.Hour).Format(time.RFC3339),
				}))
		})
	})

	t.Run("gte", func(t *testing.T) {

		t.Run("float", func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      gte,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         20,
					"probability": 0.7,
				}))

			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      gte,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         20,
					"probability": 0.5,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      gte,
						Value:         0.5,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":         25,
					"probability": 0.3,
				}))
		})

		t.Run("date", func(t *testing.T) {
			now := time.Unix(time.Now().Unix(), 0)

			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      gte,
						Value:         now,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":       20,
					"createdAt": now.Add(4 * time.Hour).Format(time.RFC3339),
				}))

			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      gte,
						Value:         now,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":       20,
					"createdAt": now.Format(time.RFC3339),
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      gte,
						Value:         now,
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"age":       25,
					"createdAt": now.Add(-6 * time.Hour).Format(time.RFC3339),
				}))
		})
	})

	t.Run("in", func(t *testing.T) {
		t.Run(filterTypeString, func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      in,
						Value:         []string{"JP", "UA"},
						FilterType:    filterTypeString,
					},
				},
				Attributes{
					"country": "JP",
					"age":     20,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      in,
						Value:         []string{"JP", "UA"},
						FilterType:    filterTypeString,
					},
				},
				Attributes{
					"country": "FR",
					"age":     20,
				}))
		})

		t.Run("float", func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      in,
						Value:         []float64{0.1, 0.2},
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         20,
					"probability": 0.2,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      in,
						Value:         []float64{0.1, 0.3},
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         20,
					"probability": 0.2,
				}))
		})

		t.Run("date", func(t *testing.T) {

			t.Run("positive test", func(t *testing.T) {
				createdAtArr := []string{"2016-03-16T05:44:23Z"}
				assert.True(t, matchByFilters(
					[]*FlagFilter{
						{
							AttributeName: "createdAt",
							Operator:      in,
							Value:         createdAtArr,
							FilterType:    filterTypeDate,
						},
					},
					Attributes{
						"createdAt": "2016-03-16T05:44:23Z",
					}))
			})

			t.Run("negative tests", func(t *testing.T) {
				t.Run("client's value is an array", func(t *testing.T) {
					createdAtArr := []string{"2016-03-16T05:44:23Z"}
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      in,
								Value:         createdAtArr,
								FilterType:    filterTypeDate,
							},
						},
						Attributes{
							"createdAt": []string{"2016-03-16T05:44:23Z"},
						}))
				})

				t.Run("client's value is a bool type", func(t *testing.T) {
					createdAtArr := []string{"2016-03-16T05:44:23Z"}
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      in,
								Value:         createdAtArr,
								FilterType:    filterTypeDate,
							},
						},
						Attributes{
							"createdAt": false,
						}))
				})
				t.Run("client's value is a int type", func(t *testing.T) {
					createdAtArr := []string{"2016-03-16T05:44:23Z"}
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      in,
								Value:         createdAtArr,
								FilterType:    filterTypeDate,
							},
						},
						Attributes{
							"createdAt": 711185707000,
						}))
				})
				t.Run("client's value is an empty string array", func(t *testing.T) {
					createdAtArr := []string{"2016-03-16T05:44:23Z"}
					assert.False(t, matchByFilters(
						[]*FlagFilter{
							{
								AttributeName: "createdAt",
								Operator:      in,
								Value:         createdAtArr,
								FilterType:    filterTypeDate,
							},
						},
						Attributes{
							"createdAt": []string{},
						}))
				})

			})

			now1 := time.Now().Add(time.Duration(rand.Intn(1000)+1) * time.Second).Truncate(time.Second)
			now2 := now1.Add(time.Duration(rand.Intn(1000)+1) * time.Second).Truncate(time.Second)

			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      in,
						Value:         []time.Time{now1, now2},
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"country":   "JP",
					"createdAt": now1.Format(time.RFC3339),
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      in,
						Value:         []time.Time{now1, now2},
						FilterType:    filterTypeDate,
					},
				},
				Attributes{
					"country":   "FR",
					"createdAt": now2.Add(time.Duration(rand.Intn(1000)+1) * time.Second),
				}))
		})
	})

	t.Run("not_in", func(t *testing.T) {
		t.Run(filterTypeString, func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      notIn,
						Value:         []string{"JP", "UA"},
						FilterType:    filterTypeString,
					},
				},
				Attributes{
					"country": "FR",
					"age":     20,
				}))

			// positive, no attribute country
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      notIn,
						Value:         []string{"JP", "UA"},
						FilterType:    filterTypeString,
					},
				},
				Attributes{
					"age": 20,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      notIn,
						Value:         []string{"JP", "UA"},
						FilterType:    filterTypeString,
					},
				},
				Attributes{
					"country": "JP",
					"age":     20,
				}))
		})

		t.Run("float", func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      notIn,
						Value:         []float64{0.1, 0.2},
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         20,
					"probability": 0.3,
				}))

			// positive, no attribute probability
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      notIn,
						Value:         []float64{0.1, 0.2},
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"country": "JP",
					"age":     20,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "probability",
						Operator:      notIn,
						Value:         []float64{0.1, 0.3},
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         20,
					"probability": 0.1,
				}))
		})

		t.Run("bool", func(t *testing.T) {
			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "admin",
						Operator:      notIn,
						Value:         []bool{true},
						FilterType:    filterTypeBool,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         20,
					"probability": 0.3,
					"admin":       false,
				}))

			// positive, no attribute admin
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "admin",
						Operator:      notIn,
						Value:         []bool{true},
						FilterType:    filterTypeBool,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         20,
					"probability": 0.3,
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "admin",
						Operator:      notIn,
						Value:         []bool{true},
						FilterType:    filterTypeBool,
					},
				},
				Attributes{
					"country":     "JP",
					"age":         20,
					"probability": 0.1,
					"admin":       true,
				}))
		})

		t.Run("date", func(t *testing.T) {
			now1 := randTS()
			now2 := randTSNEq(now1)
			now3 := randTSNEq(now2)

			// positive
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      notIn,
						Value:         []string{now1.Format(time.RFC3339), now2.Format(time.RFC3339)},
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"country":   "JP",
					"createdAt": now3.Format(time.RFC3339),
				}))

			// positive, no attribute age
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      notIn,
						Value:         []string{now1.Format(time.RFC3339), now2.Format(time.RFC3339)},
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"country": "JP",
				}))

			// negative
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "createdAt",
						Operator:      notIn,
						Value:         []string{now1.Format(time.RFC3339), now2.Format(time.RFC3339)},
						FilterType:    filterTypeNumber,
					},
				},
				Attributes{
					"country":   "FR",
					"createdAt": now2.Format(time.RFC3339),
				}))
		})
	})

	t.Run("composite is", func(t *testing.T) {
		now := time.Unix(time.Now().Unix(), 0)

		// positive
		assert.True(t, matchByFilters(
			[]*FlagFilter{
				{
					AttributeName: "country",
					Operator:      is,
					Value:         "JP",
					FilterType:    filterTypeString,
				}, {
					AttributeName: "age",
					Operator:      is,
					Value:         21.0,
					FilterType:    filterTypeNumber,
				},
				{
					AttributeName: "fire",
					Operator:      is,
					Value:         true,
					FilterType:    filterTypeBool,
				}, {
					AttributeName: "createdAt",
					Operator:      lt,
					Value:         now,
					FilterType:    filterTypeDate,
				}},
			Attributes{
				"country":   "JP",
				"age":       21,
				"fire":      true,
				"createdAt": now.Add(-100 * time.Hour).Format(time.RFC3339),
			}))

		// negative
		assert.False(t, matchByFilters(
			[]*FlagFilter{
				{
					AttributeName: "country",
					Operator:      is,
					Value:         "JP",
					FilterType:    filterTypeString,
				}, {
					AttributeName: "age",
					Operator:      is,
					Value:         21,
					FilterType:    filterTypeNumber,
				},
				{
					AttributeName: "fire",
					Operator:      is,
					Value:         true,
					FilterType:    filterTypeBool,
				}},
			Attributes{
				"country": "FR",
				"age":     36,
				"fire":    true,
			}))
	})

	t.Run("composite is_not", func(t *testing.T) {

		t.Run("pos1", func(t *testing.T) {
			country := randCountry()
			fire := randBool()
			assert.True(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      isNot,
						Value:         country,
						FilterType:    filterTypeString,
					},
					{
						AttributeName: "age",
						Operator:      isNot,
						Value:         21.0,
						FilterType:    filterTypeNumber,
					},
					{
						AttributeName: "fire",
						Operator:      isNot,
						Value:         fire,
						FilterType:    filterTypeBool,
					},
				},
				Attributes{
					"country": randCountryNEq(country),
					"age":     22,
					"fire":    randBoolNEq(fire),
				}))
		})

		t.Run("neg1", func(t *testing.T) {
			country := randCountry()
			fire := randBool()
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      isNot,
						Value:         country,
						FilterType:    filterTypeString,
					},
					{
						AttributeName: "age",
						Operator:      isNot,
						Value:         randInt(),
						FilterType:    filterTypeNumber,
					},
					{
						AttributeName: "fire",
						Operator:      isNot,
						Value:         fire,
						FilterType:    filterTypeBool,
					},
				},
				Attributes{
					"country": country,
				}))
		})

		t.Run("neg2", func(t *testing.T) {
			country := randCountry()
			age := randInt()
			fire := randBool()
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      isNot,
						Value:         country,
						FilterType:    filterTypeString,
					},
					{
						AttributeName: "age",
						Operator:      isNot,
						Value:         age,
						FilterType:    filterTypeNumber,
					},
					{
						AttributeName: "fire",
						Operator:      isNot,
						Value:         fire,
						FilterType:    filterTypeBool,
					},
				},
				Attributes{
					"country": randCountryNEq(country),
					"age":     age,
					"fire":    randBoolNEq(fire),
				}))
		})

		t.Run("neg3", func(t *testing.T) {
			country := randCountry()
			age := randInt()
			fire := randBool()
			assert.False(t, matchByFilters(
				[]*FlagFilter{
					{
						AttributeName: "country",
						Operator:      isNot,
						Value:         country,
						FilterType:    filterTypeString,
					},
					{
						AttributeName: "age",
						Operator:      isNot,
						Value:         age,
						FilterType:    filterTypeNumber,
					},
					{
						AttributeName: "fire",
						Operator:      isNot,
						Value:         fire,
						FilterType:    filterTypeBool,
					},
				},
				randAttributes(
					add("country", randCountryNEq(country)),
					add("age", randIntNEq(age)),
					add("fire", fire),
				)))
		})
	})
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func add(k string, v interface{}) func(Attributes) {
	return func(attr Attributes) {
		attr[k] = v
	}
}

func del(k string) func(Attributes) {
	return func(attr Attributes) {
		delete(attr, k)
	}
}

func randAttributes(transforms ...func(attr Attributes)) Attributes {
	attr := Attributes{}
	for k, v := range map[string]interface{}{
		"age":         randInt(),
		"fire":        randBool(),
		"country":     randCountry(),
		"createdAt":   randTS(),
		"admin":       randBool(),
		"probability": randFloat(),
	} {
		if rand.Intn(1) == 0 {
			attr[k] = v
		}
	}

	for _, f := range transforms {
		f(attr)
	}

	return attr
}

const (
	randIntMAX  = 10000
	randTimeMax = 3 * 60 * 60 * 24 // 3 days
)

var countries = []string{
	"AF", "AX", "AL", "DZ", "AS", "AD", "AO", "AI", "AQ", "AG", "AR", "AM", "AW", "AU", "AT", "AZ", "BS", "BH", "BD",
	"BB", "BY", "BE", "BZ", "BJ", "BM", "BT", "BO", "BQ", "BA", "BW", "BV", "BR", "IO", "BN", "BG", "BF", "BI", "CV",
	"KH", "CM", "CA", "KY", "CF", "TD", "CL", "CN", "CX", "CC", "CO", "KM", "CG", "CD", "CK", "CR", "CI", "HR", "CU",
	"CW", "CY", "CZ", "DK", "DJ", "DM", "DO", "EC", "EG", "SV", "GQ", "ER", "EE", "SZ", "ET", "FK", "FO", "FJ", "FI",
	"FR", "GF", "PF", "TF", "GA", "GM", "GE", "DE", "GH", "GI", "GR", "GL", "GD", "GP", "GU", "GT", "GG", "GN", "GW",
	"GY", "HT", "HM", "VA", "HN", "HK", "HU", "IS", "IN", "ID", "IR", "IQ", "IE", "IM", "IL", "IT", "JM", "JP", "JE",
	"JO", "KZ", "KE", "KI", "KP", "KR", "KW", "KG", "LA", "LV", "LB", "LS", "LR", "LY", "LI", "LT", "LU", "MO", "MG",
	"MW", "MY", "MV", "ML", "MT", "MH", "MQ", "MR", "MU", "YT", "MX", "FM", "MD", "MC", "MN", "ME", "MS", "MA", "MZ",
	"MM", "NA", "NR", "NP", "NL", "NC", "NZ", "NI", "NE", "NG", "NU", "NF", "MK", "MP", "NO", "OM", "PK", "PW", "PS",
	"PA", "PG", "PY", "PE", "PH", "PN", "PL", "PT", "PR", "QA", "RE", "RO", "RU", "RW", "BL", "SH", "KN", "LC", "MF",
	"PM", "VC", "WS", "SM", "ST", "SA", "SN", "RS", "SC", "SL", "SG", "SX", "SK", "SI", "SB", "SO", "ZA", "GS", "SS",
	"ES", "LK", "SD", "SR", "SJ", "SE", "CH", "SY", "TW", "TJ", "TZ", "TH", "TL", "TG", "TK", "TO", "TT", "TN", "TR",
	"TM", "TC", "TV", "UG", "UA", "AE", "GB", "US", "UM", "UY", "UZ", "VU", "VE", "VN", "VG", "VI", "WF", "EH", "YE",
	"ZM", "ZW",
}

func randInt() int {
	return rand.Intn(randIntMAX-1) + 1
}

func randIntNEq(v int) int {
	v2 := randInt()
	if v == v2 {
		return randIntNEq(v)
	}
	return v2
}

func randFloat() float64 {
	return math.Round(rand.Float64()*1000) / 1000
}

func randFloatNEq(v float64) float64 {
	v2 := randFloat()
	if v == v2 {
		return randFloatNEq(v)
	}
	return v2
}

func randBool() bool {
	return rand.Intn(1) == 0
}

func randBoolNEq(v bool) bool {
	return !v
}

func randTS() time.Time {
	return time.Now().Add(time.Duration(rand.Intn(randTimeMax-1)+1) * time.Second).Truncate(time.Second)
}

func randTSNEq(v time.Time) time.Time {
	v2 := randTS()
	if v.Equal(v2) {
		return randTSNEq(v)
	}
	return v2
}

func randCountry() string {
	return countries[rand.Intn(len(countries))]
}

func randCountryNEq(v string) string {
	v2 := randCountry()
	if v == v2 {
		return randCountryNEq(v)
	}
	return v2
}

func Test_int_vs_int64(t *testing.T) {
	buf := []byte(`{"a":22,"b":44444444444}`)
	var mm map[string]interface{}
	err := json.Unmarshal(buf, &mm)
	require.NoError(t, err)

	for k, v := range mm {
		log.Printf("%s: %T %+v", k, v, v)

		x1, ok := v.(int)
		if ok {
			mm[k] = x1
			log.Printf("ok, int: %d", x1)
			continue
		}

		x2, ok := v.(int64)
		if ok {
			mm[k] = x2
			log.Printf("ok, int64: %d", x1)
			continue
		}
	}

	switch v := mm["a"]; v.(type) {
	case int:
		log.Printf("int:   %+v", v)
	case int64:
		log.Printf("int64: %+v", v)
	default:
		log.Printf("def:   %+v", v)
	}
}
