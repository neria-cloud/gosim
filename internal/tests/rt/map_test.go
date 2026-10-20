package behavior_test

import (
	"cmp"
	"maps"
	"reflect"
	"slices"
	"testing"
)

const (
	mapKeyOne       = 1
	mapKeyTwo       = 2
	mapKeyThree     = 3
	mapKeyFour      = 4
	mapKeyFive      = 5
	mapKeySix       = 6
	mapKeySeven     = 7
	mapKeyEight     = 8
	mapKeyNine      = 9
	mapKeyEleven    = 11
	mapKeyThirteen  = 13
	mapKeySixteen   = 16
	mapKeySeventeen = 17
	mapKeyNineteen  = 19
	mapKeyThirtyTwo = 32

	mapValueOne         = 1
	mapValueTwo         = 2
	mapValueThree       = 3
	mapValueFive        = 5
	mapValueEight       = 8
	mapValueNine        = 9
	mapValueEleven      = 11
	mapValueTen         = 10
	mapValueSeven       = 7
	mapValueTwelve      = 12
	mapValueThirteen    = 13
	mapValueNegativeOne = -1
	mapRMWResult        = 6
	mapLengthTwo        = 2
	mapLengthFour       = 4
	mapLengthFive       = 5
	mapMissingKey       = 10
	mapZeroValue        = 0
	mapIterationTrials  = 20
	mapGrowValueOffset  = 6
	mapBadGet           = "bad get"
	mapBadLen           = "bad len"
	mapBad              = "bad"
)

func TestMapSmallGetOK(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if value, ok := m[mapKeyTwo]; !ok || value != mapValueFive {
		t.Error(mapBadGet)
	}
}

func TestMapGetOK(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyNine] = mapValueEleven
	m[mapKeySixteen] = mapValueSeven

	if value, ok := m[mapKeyTwo]; !ok || value != mapValueFive {
		t.Error(mapBadGet)
	}
	if value, ok := m[mapKeySixteen]; !ok || value != mapValueSeven {
		t.Error(mapBadGet)
	}
}

func TestMapRMW(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] += mapValueFive

	m[mapKeyOne]++

	if value, ok := m[mapKeyOne]; !ok || value != mapRMWResult {
		t.Error(mapBadGet)
	}
}

func TestMapSmallLen(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	delete(m, mapKeyTwo)

	if len(m) != mapLengthTwo {
		t.Error(mapBadLen)
	}
}

func TestMapLen(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyNine] = mapValueEleven
	m[mapKeySixteen] = mapValueSeven
	m[mapKeyThirtyTwo] = mapValueNegativeOne
	delete(m, mapKeyTwo)

	if len(m) != mapLengthFive {
		t.Error(mapBadLen)
	}

	delete(m, mapKeySixteen)
	if len(m) != mapLengthFour {
		t.Error(mapBadLen)
	}
}

func TestMapGetNil(t *testing.T) {
	var m map[int]int

	if m[mapMissingKey] != mapZeroValue {
		t.Error(mapBadGet)
	}

	if _, ok := m[mapMissingKey]; ok {
		t.Error(mapBadGet)
	}
}

func TestMapLenNil(t *testing.T) {
	var m map[int]int

	if len(m) != mapZeroValue {
		t.Error(mapBadLen)
	}
}

func TestMapIterNil(t *testing.T) {
	var m map[int]int

	for k, v := range m {
		t.Log(k, v)
	}
}

func TestMapDeleteNil(t *testing.T) {
	var m map[int]int

	delete(m, mapZeroValue)
}

// Lifted from golang.org/x/exp/maps:

// Keys returns the keys of the map m.
// The keys will be in an indeterminate order.
func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

// Values returns the values of the map m.
// The values will be in an indeterminate order.
func Values[M ~map[K]V, K comparable, V any](m M) []V {
	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}

func TestMapsValues(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	delete(m, mapKeyTwo)

	values := Values(m)
	slices.Sort(values)

	if !reflect.DeepEqual(values, []int{mapValueThree, mapValueEight}) {
		t.Error("bad values")
	}
}

func TestMapsKeys(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	delete(m, mapKeyTwo)

	keys := Keys(m)
	slices.Sort(keys)

	if !reflect.DeepEqual(keys, []int{mapKeyOne, mapKeyThree}) {
		t.Error("bad keys")
	}
}

func TestMapsClone(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	delete(m, mapKeyTwo)

	clone := maps.Clone(m)

	for k, v := range m {
		if clone[k] != v {
			t.Error("bad value", k, v, clone[k])
		}
	}

	for k, v := range clone {
		if m[k] != v {
			t.Error("bad value", k, v, m[k])
		}
	}

	if v, ok := m[mapKeyThree]; !ok || v != mapValueEight {
		t.Error(mapBadGet)
	}

	if v, ok := clone[mapKeyThree]; !ok || v != mapValueEight {
		t.Error(mapBadGet)
	}

	delete(m, mapKeyThree)

	if _, ok := m[mapKeyThree]; ok {
		t.Error(mapBadGet)
	}

	if v, ok := clone[mapKeyThree]; !ok || v != mapValueEight {
		t.Error(mapBadGet)
	}
}

func TestMapSmallGetMissing(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if _, ok := m[mapKeyFive]; ok {
		t.Error(mapBadGet)
	}
}

func TestMapGetMissing(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyEleven] = mapValueTwo
	m[mapKeySeventeen] = mapValueNine
	m[mapKeyNineteen] = mapValueOne

	if _, ok := m[mapKeyFive]; ok {
		t.Error(mapBadGet)
	}
}

func TestMapSmallGetMissingThenSet(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if _, ok := m[mapKeyFive]; ok {
		t.Error(mapBadGet)
	}

	m[mapKeyFive] = mapValueTen

	if value, ok := m[mapKeyFive]; !ok || value != mapValueTen {
		t.Error(mapBadGet)
	}
}

func TestMapGetMissingThenSet(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyEleven] = mapValueTwo
	m[mapKeySeventeen] = mapValueNine
	m[mapKeyNineteen] = mapValueOne

	if _, ok := m[mapKeyFive]; ok {
		t.Error(mapBadGet)
	}

	m[mapKeyFive] = mapValueTen

	if value, ok := m[mapKeyFive]; !ok || value != mapValueTen {
		t.Error(mapBadGet)
	}
}

func TestMapSmallDelete(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if value, ok := m[mapKeyTwo]; !ok || value != mapValueFive {
		t.Error(mapBadGet)
	}

	delete(m, mapKeyTwo)

	if _, ok := m[mapKeyTwo]; ok {
		t.Error(mapBadGet)
	}
}

func TestMapDelete(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyEleven] = mapValueTwo
	m[mapKeySeventeen] = mapValueNine
	m[mapKeyNineteen] = mapValueOne

	if value, ok := m[mapKeyTwo]; !ok || value != mapValueFive {
		t.Error(mapBadGet)
	}

	delete(m, mapKeyTwo)

	if _, ok := m[mapKeyTwo]; ok {
		t.Error(mapBadGet)
	}
}

func TestMapSmallDeleteMissing(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if _, ok := m[mapKeyFive]; ok {
		t.Error(mapBadGet)
	}

	delete(m, mapKeyFive)

	if _, ok := m[mapKeyFive]; ok {
		t.Error(mapBadGet)
	}
}

func TestMapDeleteMissing(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyEleven] = mapValueTwo
	m[mapKeySeventeen] = mapValueNine
	m[mapKeyNineteen] = mapValueOne

	if _, ok := m[mapKeyFive]; ok {
		t.Error(mapBadGet)
	}

	delete(m, mapKeyFive)

	if _, ok := m[mapKeyFive]; ok {
		t.Error(mapBadGet)
	}
}

type pair struct{ key, value int }

func TestMapSmallSimpleIter(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyTwo, mapValueFive},
		{mapKeyThree, mapValueEight},
	}

	var seen []pair
	for k, v := range m {
		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapSimpleIter(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyEleven] = mapValueNine
	m[mapKeyThirteen] = mapValueSeven

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyTwo, mapValueFive},
		{mapKeyThree, mapValueEight},
		{mapKeyEleven, mapValueNine},
		{mapKeyThirteen, mapValueSeven},
	}

	var seen []pair
	for k, v := range m {
		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapSmallRandomIter(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	var firstSeen []pair
	for k, v := range m {
		firstSeen = append(firstSeen, pair{key: k, value: v})
	}

	success := false
	for i := 0; i < mapIterationTrials; i++ {
		var seen []pair
		for k, v := range m {
			seen = append(seen, pair{key: k, value: v})
		}

		if !reflect.DeepEqual(seen, firstSeen) {
			success = true
		}
	}

	if !success {
		t.Error(mapBad)
	}
}

func TestMapRandomIter(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyEleven] = mapValueNine
	m[mapKeyThirteen] = mapValueSeven

	var firstSeen []pair
	for k, v := range m {
		firstSeen = append(firstSeen, pair{key: k, value: v})
	}

	success := false
	for i := 0; i < mapIterationTrials; i++ {
		var seen []pair
		for k, v := range m {
			seen = append(seen, pair{key: k, value: v})
		}

		if !reflect.DeepEqual(seen, firstSeen) {
			success = true
		}
	}

	if !success {
		t.Error(mapBad)
	}
}

func TestMapLiteral(t *testing.T) {
	m := map[int]int{
		mapKeyOne:   mapValueThree,
		mapKeyTwo:   mapValueFive,
		mapKeyThree: mapValueEight,
	}

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyTwo, mapValueFive},
		{mapKeyThree, mapValueEight},
	}

	var seen []pair
	for k, v := range m {
		seen = append(seen, pair{key: k, value: v})
	}
	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapSmallIterDelete(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyFour] = mapValueTen

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyFour, mapValueTen},
	}

	var seen []pair
	for k, v := range m {
		if k == mapKeyTwo || k == mapKeyThree {
			expected = append(expected, pair{k, v})
		}

		delete(m, mapKeyTwo)
		delete(m, mapKeyThree)

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapIterDelete(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyFour] = mapValueTen
	m[mapKeyFive] = mapValueEleven
	m[mapKeySix] = mapValueThirteen

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyFour, mapValueTen},
		{mapKeyFive, mapValueEleven},
		{mapKeySix, mapValueThirteen},
	}

	var seen []pair
	for k, v := range m {
		if k == mapKeyTwo || k == mapKeyThree {
			expected = append(expected, pair{k, v})
		}

		delete(m, mapKeyTwo)
		delete(m, mapKeyThree)

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapSmallIterAdd(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyTwo, mapValueFive},
		{mapKeyThree, mapValueEight},
	}

	var seen []pair
	for k, v := range m {
		if k == mapKeyFour {
			expected = append(expected, pair{k, v})
		}

		m[mapKeyFour] = mapValueTen

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapIterAdd(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyFive] = mapValueEleven
	m[mapKeySix] = mapValueThirteen

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyTwo, mapValueFive},
		{mapKeyThree, mapValueEight},
		{mapKeyFive, mapValueEleven},
		{mapKeySix, mapValueThirteen},
	}

	var seen []pair
	for k, v := range m {
		if k == mapKeyFour {
			expected = append(expected, pair{k, v})
		}

		m[mapKeyFour] = mapValueTen

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapGrowingIterAdd(t *testing.T) {
	// grow from small to not small during iteration

	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyTwo, mapValueFive},
		{mapKeyThree, mapValueEight},
	}

	var seen []pair
	for k, v := range m {
		if k >= mapKeyFour && k <= mapKeySeven {
			expected = append(expected, pair{k, k + mapGrowValueOffset})
		}

		m[mapKeyFour] = mapValueTen
		m[mapKeyFive] = mapValueEleven
		m[mapKeySix] = mapValueTwelve
		m[mapKeySeven] = mapValueThirteen

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapIterDeleteAdd(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyFour] = mapValueTen

	expected := []pair{
		{mapKeyOne, mapValueThree},
		{mapKeyFour, mapValueTen},
	}

	var seen []pair
	for k, v := range m {
		if k == mapKeyTwo || k == mapKeyThree || k == mapKeyFive {
			expected = append(expected, pair{k, v})
		}

		delete(m, mapKeyTwo)
		delete(m, mapKeyThree)
		m[mapKeyFive] = mapValueThirteen

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapIterDeleteRestore(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	expected := []pair{}

	var seen []pair
	for k, v := range m {
		delete(m, k)
		m[k] = v

		expected = append(expected, pair{key: k, value: v})
		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapNilClear(t *testing.T) {
	var m map[string]int
	clear(m)
	if m != nil || len(m) != 0 {
		t.Fatal("clear changed a nil map")
	}
}

func TestMapSmallClear(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	clear(m)
	if len(m) != 0 {
		t.Error(mapBadLen)
	}
	if _, ok := m[mapKeyTwo]; ok {
		t.Error(mapBadGet)
	}

	m[mapKeyEight] = mapValueTen
	if value, ok := m[mapKeyEight]; !ok || value != mapValueTen {
		t.Error(mapBadGet)
	}
}

func TestMapClear(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	m[mapKeyNine] = mapValueEleven
	m[mapKeySixteen] = mapValueSeven

	clear(m)
	if len(m) != 0 {
		t.Error(mapBadLen)
	}
	if _, ok := m[mapKeyTwo]; ok {
		t.Error(mapBadGet)
	}

	m[mapKeyEight] = mapValueTen
	if value, ok := m[mapKeyEight]; !ok || value != mapValueTen {
		t.Error(mapBadGet)
	}
}

func TestMapLiteralRange(t *testing.T) {
	var seen []pair

	for k, v := range map[int]int{
		mapKeyOne:   mapValueThree,
		mapKeyTwo:   mapValueFive,
		mapKeyThree: mapValueEight,
	} {
		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	expected := []pair{
		{key: mapKeyOne, value: mapValueThree},
		{key: mapKeyTwo, value: mapValueFive},
		{key: mapKeyThree, value: mapValueEight},
	}

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapAliasConversions(t *testing.T) {
	type Alias map[int]int

	x := []Alias{
		map[int]int{mapKeyOne: mapValueThree},
		map[int]int{mapKeyTwo: mapValueFive},
	}

	y := [...]Alias{
		map[int]int{mapKeyThree: mapValueEight},
	}

	var seen []pair
	for _, m := range x {
		for k, v := range m {
			seen = append(seen, pair{key: k, value: v})
		}
	}
	for _, m := range y {
		for k, v := range m {
			seen = append(seen, pair{key: k, value: v})
		}
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	expected := []pair{
		{key: mapKeyOne, value: mapValueThree},
		{key: mapKeyTwo, value: mapValueFive},
		{key: mapKeyThree, value: mapValueEight},
	}

	if !reflect.DeepEqual(seen, expected) {
		t.Error(mapBad, seen, expected)
	}
}

func TestMapMultiAssign(t *testing.T) {
	m := make(map[int]string)

	var foo string
	m[mapKeyOne], m[mapKeyTwo], m[mapKeyThree], foo = "4", "5", "6", "7"

	if m[mapKeyOne] != "4" {
		t.Errorf("expected 4, got %s", m[mapKeyOne])
	}
	if m[mapKeyTwo] != "5" {
		t.Errorf("expected 5, got %s", m[mapKeyTwo])
	}
	if m[mapKeyThree] != "6" {
		t.Errorf("expected 6, got %s", m[mapKeyThree])
	}
	if foo != "7" {
		t.Errorf("expected 7, got %s", foo)
	}
}
