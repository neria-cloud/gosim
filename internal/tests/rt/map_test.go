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
	mapKeyFive      = 5
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
	mapValueNegativeOne = -1
	mapRMWResult        = 6
	mapLengthTwo        = 2
	mapLengthFour       = 4
	mapLengthFive       = 5
	mapMissingKey       = 10
	mapZeroValue        = 0
	mapIterationTrials  = 20
)

func TestMapSmallGetOK(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if value, ok := m[mapKeyTwo]; !ok || value != mapValueFive {
		t.Error("bad get")
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
		t.Error("bad get")
	}
	if value, ok := m[mapKeySixteen]; !ok || value != mapValueSeven {
		t.Error("bad get")
	}
}

func TestMapRMW(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] += mapValueFive

	m[mapKeyOne]++

	if value, ok := m[mapKeyOne]; !ok || value != mapRMWResult {
		t.Error("bad get")
	}
}

func TestMapSmallLen(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight
	delete(m, mapKeyTwo)

	if len(m) != mapLengthTwo {
		t.Error("bad len")
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
		t.Error("bad len")
	}

	delete(m, mapKeySixteen)
	if len(m) != mapLengthFour {
		t.Error("bad len")
	}
}

func TestMapGetNil(t *testing.T) {
	var m map[int]int

	if m[mapMissingKey] != mapZeroValue {
		t.Error("bad get")
	}

	if _, ok := m[mapMissingKey]; ok {
		t.Error("bad get")
	}
}

func TestMapLenNil(t *testing.T) {
	var m map[int]int

	if len(m) != mapZeroValue {
		t.Error("bad len")
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
		t.Error("bad get")
	}

	if v, ok := clone[mapKeyThree]; !ok || v != mapValueEight {
		t.Error("bad get")
	}

	delete(m, mapKeyThree)

	if _, ok := m[mapKeyThree]; ok {
		t.Error("bad get")
	}

	if v, ok := clone[mapKeyThree]; !ok || v != mapValueEight {
		t.Error("bad get")
	}
}

func TestMapSmallGetMissing(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if _, ok := m[mapKeyFive]; ok {
		t.Error("bad get")
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
		t.Error("bad get")
	}
}

func TestMapSmallGetMissingThenSet(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if _, ok := m[mapKeyFive]; ok {
		t.Error("bad get")
	}

	m[mapKeyFive] = mapValueTen

	if value, ok := m[mapKeyFive]; !ok || value != mapValueTen {
		t.Error("bad get")
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
		t.Error("bad get")
	}

	m[mapKeyFive] = mapValueTen

	if value, ok := m[mapKeyFive]; !ok || value != mapValueTen {
		t.Error("bad get")
	}
}

func TestMapSmallDelete(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if value, ok := m[mapKeyTwo]; !ok || value != mapValueFive {
		t.Error("bad get")
	}

	delete(m, mapKeyTwo)

	if _, ok := m[mapKeyTwo]; ok {
		t.Error("bad get")
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
		t.Error("bad get")
	}

	delete(m, mapKeyTwo)

	if _, ok := m[mapKeyTwo]; ok {
		t.Error("bad get")
	}
}

func TestMapSmallDeleteMissing(t *testing.T) {
	m := make(map[int]int)
	m[mapKeyOne] = mapValueThree
	m[mapKeyTwo] = mapValueFive
	m[mapKeyThree] = mapValueEight

	if _, ok := m[mapKeyFive]; ok {
		t.Error("bad get")
	}

	delete(m, mapKeyFive)

	if _, ok := m[mapKeyFive]; ok {
		t.Error("bad get")
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
		t.Error("bad get")
	}

	delete(m, mapKeyFive)

	if _, ok := m[mapKeyFive]; ok {
		t.Error("bad get")
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
		t.Error("bad", seen, expected)
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
		t.Error("bad", seen, expected)
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
		t.Error("bad")
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
		t.Error("bad")
	}
}

func TestMapLiteral(t *testing.T) {
	m := map[int]int{
		1: 3,
		2: 5,
		3: 8,
	}

	expected := []pair{
		{1, 3},
		{2, 5},
		{3, 8},
	}

	var seen []pair
	for k, v := range m {
		seen = append(seen, pair{key: k, value: v})
	}
	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapSmallIterDelete(t *testing.T) {
	m := make(map[int]int)
	m[1] = 3
	m[2] = 5
	m[3] = 8
	m[4] = 10

	expected := []pair{
		{1, 3},
		{4, 10},
	}

	var seen []pair
	for k, v := range m {
		if k == 2 || k == 3 {
			expected = append(expected, pair{k, v})
		}

		delete(m, 2)
		delete(m, 3)

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapIterDelete(t *testing.T) {
	m := make(map[int]int)
	m[1] = 3
	m[2] = 5
	m[3] = 8
	m[4] = 10
	m[5] = 11
	m[6] = 13

	expected := []pair{
		{1, 3},
		{4, 10},
		{5, 11},
		{6, 13},
	}

	var seen []pair
	for k, v := range m {
		if k == 2 || k == 3 {
			expected = append(expected, pair{k, v})
		}

		delete(m, 2)
		delete(m, 3)

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapSmallIterAdd(t *testing.T) {
	m := make(map[int]int)
	m[1] = 3
	m[2] = 5
	m[3] = 8

	expected := []pair{
		{1, 3},
		{2, 5},
		{3, 8},
	}

	var seen []pair
	for k, v := range m {
		if k == 4 {
			expected = append(expected, pair{k, v})
		}

		m[4] = 10

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapIterAdd(t *testing.T) {
	m := make(map[int]int)
	m[1] = 3
	m[2] = 5
	m[3] = 8
	m[5] = 11
	m[6] = 13

	expected := []pair{
		{1, 3},
		{2, 5},
		{3, 8},
		{5, 11},
		{6, 13},
	}

	var seen []pair
	for k, v := range m {
		if k == 4 {
			expected = append(expected, pair{k, v})
		}

		m[4] = 10

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapGrowingIterAdd(t *testing.T) {
	// grow from small to not small during iteration

	m := make(map[int]int)
	m[1] = 3
	m[2] = 5
	m[3] = 8

	expected := []pair{
		{1, 3},
		{2, 5},
		{3, 8},
	}

	var seen []pair
	for k, v := range m {
		if k >= 4 && k <= 7 {
			expected = append(expected, pair{k, k + 6})
		}

		m[4] = 10
		m[5] = 11
		m[6] = 12
		m[7] = 13

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapIterDeleteAdd(t *testing.T) {
	m := make(map[int]int)
	m[1] = 3
	m[2] = 5
	m[3] = 8
	m[4] = 10

	expected := []pair{
		{1, 3},
		{4, 10},
	}

	var seen []pair
	for k, v := range m {
		if k == 2 || k == 3 || k == 5 {
			expected = append(expected, pair{k, v})
		}

		delete(m, 2)
		delete(m, 3)
		m[5] = 13

		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})
	slices.SortFunc(expected, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapIterDeleteRestore(t *testing.T) {
	m := make(map[int]int)
	m[1] = 3
	m[2] = 5
	m[3] = 8

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
		t.Error("bad", seen, expected)
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
	m[1] = 3
	m[2] = 5
	m[3] = 8

	clear(m)
	if len(m) != 0 {
		t.Error("bad len")
	}
	if _, ok := m[2]; ok {
		t.Error("bad get")
	}

	m[8] = 10
	if value, ok := m[8]; !ok || value != 10 {
		t.Error("bad get")
	}
}

func TestMapClear(t *testing.T) {
	m := make(map[int]int)
	m[1] = 3
	m[2] = 5
	m[3] = 8
	m[9] = 11
	m[16] = 7

	clear(m)
	if len(m) != 0 {
		t.Error("bad len")
	}
	if _, ok := m[2]; ok {
		t.Error("bad get")
	}

	m[8] = 10
	if value, ok := m[8]; !ok || value != 10 {
		t.Error("bad get")
	}
}

func TestMapLiteralRange(t *testing.T) {
	var seen []pair

	for k, v := range map[int]int{
		1: 3,
		2: 5,
		3: 8,
	} {
		seen = append(seen, pair{key: k, value: v})
	}

	slices.SortFunc(seen, func(a pair, b pair) int {
		return cmp.Compare(a.key, b.key)
	})

	expected := []pair{
		{key: 1, value: 3},
		{key: 2, value: 5},
		{key: 3, value: 8},
	}

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapAliasConversions(t *testing.T) {
	type Alias map[int]int

	x := []Alias{
		map[int]int{1: 3},
		map[int]int{2: 5},
	}

	y := [...]Alias{
		map[int]int{3: 8},
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
		{key: 1, value: 3},
		{key: 2, value: 5},
		{key: 3, value: 8},
	}

	if !reflect.DeepEqual(seen, expected) {
		t.Error("bad", seen, expected)
	}
}

func TestMapMultiAssign(t *testing.T) {
	m := make(map[int]string)

	var foo string
	m[1], m[2], m[3], foo = "4", "5", "6", "7"

	if m[1] != "4" {
		t.Errorf("expected 4, got %s", m[1])
	}
	if m[2] != "5" {
		t.Errorf("expected 5, got %s", m[2])
	}
	if m[3] != "6" {
		t.Errorf("expected 6, got %s", m[3])
	}
	if foo != "7" {
		t.Errorf("expected 7, got %s", foo)
	}
}
