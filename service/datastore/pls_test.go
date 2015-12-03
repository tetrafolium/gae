// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// adapted from github.com/golang/appengine/datastore

package datastore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/tetrafolium/gae/service/blobstore"
)

var (
	mp   = MkProperty
	mpNI = MkPropertyNI
)

const testAppID = "testApp"

type (
	myBlob   []byte
	myByte   byte
	myString string
)

func makeMyByteSlice(n int) []myByte {
	b := make([]myByte, n)
	for i := range b {
		b[i] = myByte(i)
	}
	return b
}

func makeInt8Slice(n int) []int8 {
	b := make([]int8, n)
	for i := range b {
		b[i] = int8(i)
	}
	return b
}

func makeUint8Slice(n int) []uint8 {
	b := make([]uint8, n)
	for i := range b {
		b[i] = uint8(i)
	}
	return b
}

var (
	testKey0     = mkKey("aid", "", "kind", "name0")
	testKey1a    = mkKey("aid", "", "kind", "name1")
	testKey1b    = mkKey("aid", "", "kind", "name1")
	testKey2a    = mkKey("aid", "", "kind", "name0", "kind", "name2")
	testKey2b    = mkKey("aid", "", "kind", "name0", "kind", "name2")
	testGeoPt0   = GeoPoint{Lat: 1.2, Lng: 3.4}
	testGeoPt1   = GeoPoint{Lat: 5, Lng: 10}
	testBadGeoPt = GeoPoint{Lat: 1000, Lng: 34}
)

type B0 struct {
	B []byte `gae:",noindex"`
}

type B1 struct {
	B []int8
}

type B2 struct {
	B myBlob
}

type B3 struct {
	B []myByte
}

type B4 struct {
	B [][]byte `gae:",noindex"`
}

type B5 struct {
	B []byte
}

type C0 struct {
	I int
	C chan int
}

type C1 struct {
	I int
	C *chan int
}

type C2 struct {
	I int
	C []chan int
}

type C3 struct {
	C string
}

type E struct{}

type G0 struct {
	G GeoPoint
}

type G1 struct {
	G []GeoPoint
}

type K0 struct {
	K *Key
}

type K1 struct {
	K []*Key
}

type N0 struct {
	X0
	ID       int64  `gae:"$id"`
	_kind    string `gae:"$kind,whatnow"`
	Nonymous X0
	Ignore   string `gae:"-"`
	Other    string
}

type N1 struct {
	X0
	Nonymous []X0
	Ignore   string `gae:"-"`
	Other    string
}

type N2 struct {
	N1    `gae:"red"`
	Green N1 `gae:"green"`
	Blue  N1
	White N1 `gae:"-"`
}

type O0 struct {
	I int64
}

type O1 struct {
	I int32
}

type U0 struct {
	U uint
}

type U1 struct {
	U string
}

type T struct {
	T time.Time
}

type X0 struct {
	S string
	I int
	i int
}

type X1 struct {
	S myString
	I int32
	J int64
}

type X2 struct {
	Z string
	i int
}

type X3 struct {
	S bool
	I int
}

type Y0 struct {
	B bool
	F []float64
	G []float64
}

type Y1 struct {
	B bool
	F float64
}

type Y2 struct {
	B bool
	F []int64
}

type Y3 struct {
	B bool
	F int64
}

type Tagged struct {
	A int   `gae:"a,noindex"`
	B []int `gae:"b1"`
	C int   `gae:",noindex"`
	D int   `gae:""`
	E int
	I int `gae:"-"`
	J int `gae:",noindex" json:"j"`

	Y0 `gae:"-"`
	Z  chan int `gae:"-,"`
}

type InvalidTagged1 struct {
	I int `gae:"\t"`
}

type InvalidTagged2 struct {
	I int
	J int `gae:"I"`
}

type InvalidTagged3 struct {
	I int `gae:"a\t"`
}

type InvalidTagged4 struct {
	I int `gae:"a."`
}

type InvalidTaggedSub struct {
	I int
}

type InvalidTagged5 struct {
	I int `gae:"V.I"`
	V []InvalidTaggedSub
}

type Inner1 struct {
	W int32
	X string
}

type Inner2 struct {
	Y float64
}

type Inner3 struct {
	Z bool
}

type Outer struct {
	A int16
	I []Inner1
	J Inner2
	Inner3
}

type OuterEquivalent struct {
	A     int16
	IDotW []int32  `gae:"I.W"`
	IDotX []string `gae:"I.X"`
	JDotY float64  `gae:"J.Y"`
	Z     bool
}

type Dotted struct {
	A DottedA `gae:"A0.A1.A2"`
}

type DottedA struct {
	B DottedB `gae:"B3"`
}

type DottedB struct {
	C int `gae:"C4.C5"`
}

type SliceOfSlices struct {
	I int
	S []struct {
		J int
		F []float64
	}
}

type Recursive struct {
	I int
	R []Recursive
}

type MutuallyRecursive0 struct {
	I int
	R []MutuallyRecursive1
}

type MutuallyRecursive1 struct {
	I int
	R []MutuallyRecursive0
}

type ExoticTypes struct {
	BS blobstore.Key
}

type Underspecified struct {
	Iface PropertyConverter
}

type MismatchTypes struct {
	S  string
	B  bool
	F  float32
	K  *Key
	T  time.Time
	G  GeoPoint
	IS []int
}

type BadMeta struct {
	ID int64  `gae:"$id"`
	id string `gae:"$id"`
}

type Doubler struct {
	S string
	I int64
	B bool
}

func (d *Doubler) Load(props PropertyMap) error {
	return GetPLS(d).Load(props)
}

func (d *Doubler) Save(withMeta bool) (PropertyMap, error) {
	pls := GetPLS(d)
	propMap, err := pls.Save(withMeta)
	if err != nil {
		return nil, err
	}

	// Edit that map and send it on.
	for _, props := range propMap {
		for i := range props {
			switch v := props[i].Value().(type) {
			case string:
				// + means string concatenation.
				So(props[i].SetValue(v+v, props[i].IndexSetting()), ShouldBeNil)
			case int64:
				// + means integer addition.
				So(props[i].SetValue(v+v, props[i].IndexSetting()), ShouldBeNil)
			}
		}
	}
	return propMap, nil
}

func (d *Doubler) Problem() error { return nil }

var _ PropertyLoadSaver = (*Doubler)(nil)

type Deriver struct {
	S, Derived, Ignored string
}

func (d *Deriver) Load(props PropertyMap) error {
	for name, p := range props {
		if name != "S" {
			continue
		}
		d.S = p[0].Value().(string)
		d.Derived = "derived+" + d.S
	}
	return nil
}

func (d *Deriver) Save(withMeta bool) (PropertyMap, error) {
	return map[string][]Property{
		"S": {mp(d.S)},
	}, nil
}

func (d *Deriver) Problem() error { return nil }

var _ PropertyLoadSaver = (*Deriver)(nil)

type Augmenter struct {
	S string

	g string `gae:"-"`
}

func (a *Augmenter) Load(props PropertyMap) error {
	if e := props["Extra"]; len(e) > 0 {
		a.g = e[0].Value().(string)
		delete(props, "Extra")
	}
	if err := GetPLS(a).Load(props); err != nil {
		return err
	}
	return nil
}

func (a *Augmenter) Save(withMeta bool) (PropertyMap, error) {
	props, err := GetPLS(a).Save(withMeta)
	if err != nil {
		return nil, err
	}
	props["Extra"] = []Property{MkProperty("ohai!")}
	return props, nil
}

func (a *Augmenter) Problem() error { return nil }

var _ PropertyLoadSaver = (*Augmenter)(nil)

type BK struct {
	Key blobstore.Key
}

type Convertable []int64

var _ PropertyConverter = (*Convertable)(nil)

func (c *Convertable) ToProperty() (ret Property, err error) {
	buf := make([]string, len(*c))
	for i, v := range *c {
		buf[i] = strconv.FormatInt(v, 10)
	}
	err = ret.SetValue(strings.Join(buf, ","), NoIndex)
	return
}

func (c *Convertable) FromProperty(pv Property) error {
	if sval, ok := pv.Value().(string); ok {
		for _, t := range strings.Split(sval, ",") {
			ival, err := strconv.ParseInt(t, 10, 64)
			if err != nil {
				return err
			}
			*c = append(*c, ival)
		}
		return nil
	}
	return fmt.Errorf("nope")
}

type Impossible struct {
	Nested []ImpossibleInner
}

type ImpossibleInner struct {
	Ints Convertable `gae:"wot"`
}

type Convertable2 struct {
	Data     string
	Exploded []string
}

func (c *Convertable2) ToProperty() (ret Property, err error) {
	err = ret.SetValue(c.Data, ShouldIndex)
	return
}

func (c *Convertable2) FromProperty(pv Property) error {
	if sval, ok := pv.Value().(string); ok {
		c.Data = sval
		c.Exploded = []string{"turn", "down", "for", "what"}
		return nil
	}
	return fmt.Errorf("nope")
}

type Impossible2 struct {
	Nested []ImpossibleInner2
}

type ImpossibleInner2 struct {
	Thingy Convertable2 `gae:"nerb"`
}

type JSONKVProp map[string]interface{}

var _ PropertyConverter = (*JSONKVProp)(nil)

func (j *JSONKVProp) ToProperty() (ret Property, err error) {
	data, err := json.Marshal(map[string]interface{}(*j))
	if err != nil {
		return
	}
	err = ret.SetValue(data, NoIndex)
	return
}

func (j *JSONKVProp) FromProperty(pv Property) error {
	if bval, ok := pv.Value().([]byte); ok {
		dec := json.NewDecoder(bytes.NewBuffer(bval))
		dec.UseNumber()
		return dec.Decode((*map[string]interface{})(j))
	}
	return fmt.Errorf("nope")
}

type Impossible3 struct {
	KMap JSONKVProp `gae:"kewelmap"`
}

type Complex complex128

var _ PropertyConverter = (*Complex)(nil)

func (c *Complex) ToProperty() (ret Property, err error) {
	// cheat hardkore and usurp GeoPoint so datastore will index these suckers
	// (note that this won't REALLY work, since GeoPoints are limited to a very
	// limited range of values, but it's nice to pretend ;)). You'd probably
	// really end up with a packed binary representation.
	err = ret.SetValue(GeoPoint{Lat: real(*c), Lng: imag(*c)}, ShouldIndex)
	return
}

func (c *Complex) FromProperty(p Property) error {
	if gval, ok := p.Value().(GeoPoint); ok {
		*c = Complex(complex(gval.Lat, gval.Lng))
		return nil
	}
	return fmt.Errorf("nope")
}

type Impossible4 struct {
	Values []Complex
}

type DerivedKey struct {
	K *Key
}

type IfaceKey struct {
	K *Key
}

type IDParser struct {
	_kind string `gae:"$kind,CoolKind"`

	// real $id is myParentName|myID
	parent string `gae:"-"`
	id     int64  `gae:"-"`
}

var _ MetaGetterSetter = (*IDParser)(nil)

func (i *IDParser) getFullID() string {
	return fmt.Sprintf("%s|%d", i.parent, i.id)
}

func (i *IDParser) GetAllMeta() PropertyMap {
	pm := GetPLS(i).GetAllMeta()
	pm.SetMeta("id", i.getFullID())
	return pm
}

func (i *IDParser) GetMeta(key string) (interface{}, error) {
	if key == "id" {
		return i.getFullID(), nil
	}
	return GetPLS(i).GetMeta(key)
}

func (i *IDParser) GetMetaDefault(key string, dflt interface{}) interface{} {
	return GetMetaDefaultImpl(i.GetMeta, key, dflt)
}

func (i *IDParser) SetMeta(key string, value interface{}) (err error) {
	if key == "id" {
		// let the panics flooowwww
		vS := strings.SplitN(value.(string), "|", 2)
		i.parent = vS[0]
		i.id, err = strconv.ParseInt(vS[1], 10, 64)
		return
	}
	return GetPLS(i).SetMeta(key, value)
}

type KindOverride struct {
	ID int64 `gae:"$id"`

	customKind string `gae:"-"`
}

var _ MetaGetterSetter = (*KindOverride)(nil)

func (i *KindOverride) GetAllMeta() PropertyMap {
	pm := GetPLS(i).GetAllMeta()
	if i.customKind != "" {
		pm.SetMeta("kind", i.customKind)
	}
	return pm
}

func (i *KindOverride) GetMeta(key string) (interface{}, error) {
	if key == "kind" && i.customKind != "" {
		return i.customKind, nil
	}
	return GetPLS(i).GetMeta(key)
}

func (i *KindOverride) GetMetaDefault(key string, dflt interface{}) interface{} {
	return GetMetaDefaultImpl(i.GetMeta, key, dflt)
}

func (i *KindOverride) SetMeta(key string, value interface{}) error {
	if key == "kind" {
		kind := value.(string)
		if kind != "KindOverride" {
			i.customKind = kind
		} else {
			i.customKind = ""
		}
		return nil
	}
	return ErrMetaFieldUnset
}

type EmbeddedID struct {
	Thing string
	Val   int
}

var _ PropertyConverter = (*EmbeddedID)(nil)

func (e *EmbeddedID) ToProperty() (ret Property, err error) {
	return mpNI(fmt.Sprintf("%s|%d", e.Thing, e.Val)), nil
}

func (e *EmbeddedID) FromProperty(val Property) error {
	if val.Type() != PTString {
		return fmt.Errorf("gotta have a string")
	}
	toks := strings.SplitN(val.Value().(string), "|", 2)
	if len(toks) != 2 {
		return fmt.Errorf("gotta have two parts")
	}
	v, err := strconv.Atoi(toks[1])
	if err != nil {
		return err
	}

	e.Thing = toks[0]
	e.Val = v
	return nil
}

type IDEmbedder struct {
	EmbeddedID `gae:"$id"`
}

type Simple struct{}

type testCase struct {
	desc       string
	src        interface{}
	want       interface{}
	plsErr     string
	saveErr    string
	plsLoadErr string
	loadErr    string
}

var testCases = []testCase{
	{
		desc:   "chan save fails",
		src:    &C0{I: -1},
		plsErr: `field "C" has invalid type: chan int`,
	},
	{
		desc:   "*chan save fails",
		src:    &C1{I: -1},
		plsErr: `field "C" has invalid type: *chan int`,
	},
	{
		desc:   "[]chan save fails",
		src:    &C2{I: -1, C: make([]chan int, 8)},
		plsErr: `field "C" has invalid type: []chan int`,
	},
	{
		desc:       "chan load fails",
		src:        &C3{C: "not a chan"},
		want:       &C0{},
		plsLoadErr: `field "C" has invalid type: chan int`,
	},
	{
		desc:       "*chan load fails",
		src:        &C3{C: "not a *chan"},
		want:       &C1{},
		plsLoadErr: `field "C" has invalid type: *chan int`,
	},
	{
		desc:       "[]chan load fails",
		src:        &C3{C: "not a []chan"},
		want:       &C2{},
		plsLoadErr: `field "C" has invalid type: []chan int`,
	},
	{
		desc: "empty struct",
		src:  &E{},
		want: &E{},
	},
	{
		desc: "geopoint",
		src:  &G0{G: testGeoPt0},
		want: &G0{G: testGeoPt0},
	},
	{
		desc:    "geopoint invalid",
		src:     &G0{G: testBadGeoPt},
		saveErr: "invalid GeoPoint value",
	},
	{
		desc: "geopoint as props",
		src:  &G0{G: testGeoPt0},
		want: PropertyMap{
			"G": {mp(testGeoPt0)},
		},
	},
	{
		desc: "geopoint slice",
		src:  &G1{G: []GeoPoint{testGeoPt0, testGeoPt1}},
		want: &G1{G: []GeoPoint{testGeoPt0, testGeoPt1}},
	},
	{
		desc: "key",
		src:  &K0{K: testKey1a},
		want: &K0{K: testKey1b},
	},
	{
		desc: "key with parent",
		src:  &K0{K: testKey2a},
		want: &K0{K: testKey2b},
	},
	{
		desc: "nil key",
		src:  &K0{},
		want: &K0{},
	},
	{
		desc: "all nil keys in slice",
		src:  &K1{[]*Key{nil, nil}},
		want: &K1{[]*Key{nil, nil}},
	},
	{
		desc: "some nil keys in slice",
		src:  &K1{[]*Key{testKey1a, nil, testKey2a}},
		want: &K1{[]*Key{testKey1b, nil, testKey2b}},
	},
	{
		desc:    "overflow",
		src:     &O0{I: 1 << 48},
		want:    &O1{},
		loadErr: "overflow",
	},
	{
		desc: "time",
		src:  &T{T: time.Unix(1e9, 0).UTC()},
		want: &T{T: time.Unix(1e9, 0).UTC()},
	},
	{
		desc: "time as props",
		src:  &T{T: time.Unix(1e9, 0).UTC()},
		want: PropertyMap{
			"T": {mp(time.Unix(1e9, 0).UTC())},
		},
	},
	{
		desc:   "uint save",
		src:    &U0{U: 1},
		plsErr: `field "U" has invalid type: uint`,
	},
	{
		desc:       "uint load",
		src:        &U1{U: "not a uint"},
		want:       &U0{},
		plsLoadErr: `field "U" has invalid type: uint`,
	},
	{
		desc: "zero",
		src:  &X0{},
		want: &X0{},
	},
	{
		desc: "basic",
		src:  &X0{S: "one", I: 2, i: 3},
		want: &X0{S: "one", I: 2},
	},
	{
		desc: "save string/int load myString/int32",
		src:  &X0{S: "one", I: 2, i: 3},
		want: &X1{S: "one", I: 2},
	},
	{
		desc:    "missing fields",
		src:     &X0{S: "one", I: 2, i: 3},
		want:    &X2{},
		loadErr: "no such struct field",
	},
	{
		desc:    "save string load bool",
		src:     &X0{S: "one", I: 2, i: 3},
		want:    &X3{I: 2},
		loadErr: "type mismatch",
	},
	{
		desc: "basic slice",
		src:  &Y0{B: true, F: []float64{7, 8, 9}},
		want: &Y0{B: true, F: []float64{7, 8, 9}},
	},
	{
		desc:    "save []float64 load float64",
		src:     &Y0{B: true, F: []float64{7, 8, 9}},
		want:    &Y1{B: true},
		loadErr: "requires a slice",
	},
	{
		desc: "save single []int64 load int64",
		src:  &Y2{B: true, F: []int64{7}},
		want: &Y3{B: true, F: 7},
	},
	{
		desc: "save int64 load single []int64",
		src:  &Y3{B: true, F: 7},
		want: &Y2{B: true, F: []int64{7}},
	},
	{
		desc: "use convertable slice",
		src:  &Impossible{[]ImpossibleInner{{Convertable{1, 5, 9}}, {Convertable{2, 4, 6}}}},
		want: &Impossible{[]ImpossibleInner{{Convertable{1, 5, 9}}, {Convertable{2, 4, 6}}}},
	},
	{
		desc: "use convertable slice (to map)",
		src:  &Impossible{[]ImpossibleInner{{Convertable{1, 5, 9}}, {Convertable{2, 4, 6}}}},
		want: PropertyMap{
			"Nested.wot": {mpNI("1,5,9"), mpNI("2,4,6")},
		},
	},
	{
		desc:    "convertable slice (bad load)",
		src:     PropertyMap{"Nested.wot": {mpNI([]byte("ohai"))}},
		want:    &Impossible{[]ImpossibleInner{{}}},
		loadErr: "nope",
	},
	{
		desc: "use convertable struct",
		src: &Impossible2{
			[]ImpossibleInner2{
				{Convertable2{"nerb", nil}},
			},
		},
		want: &Impossible2{
			[]ImpossibleInner2{
				{Convertable2{"nerb", []string{"turn", "down", "for", "what"}}},
			},
		},
	},
	{
		desc: "convertable json KVMap",
		src: &Impossible3{
			JSONKVProp{
				"epic":    "success",
				"no_way!": []interface{}{true, "story"},
				"what":    []interface{}{"is", "really", 100},
			},
		},
		want: &Impossible3{
			JSONKVProp{
				"epic":    "success",
				"no_way!": []interface{}{true, "story"},
				"what":    []interface{}{"is", "really", json.Number("100")},
			},
		},
	},
	{
		desc: "convertable json KVMap (to map)",
		src: &Impossible3{
			JSONKVProp{
				"epic":    "success",
				"no_way!": []interface{}{true, "story"},
				"what":    []interface{}{"is", "really", 100},
			},
		},
		want: PropertyMap{
			"kewelmap": {
				mpNI([]byte(
					`{"epic":"success","no_way!":[true,"story"],"what":["is","really",100]}`))},
		},
	},
	{
		desc: "convertable complex slice",
		src: &Impossible4{
			[]Complex{complex(1, 2), complex(3, 4)},
		},
		want: &Impossible4{
			[]Complex{complex(1, 2), complex(3, 4)},
		},
	},
	{
		desc: "convertable complex slice (to map)",
		src: &Impossible4{
			[]Complex{complex(1, 2), complex(3, 4)},
		},
		want: PropertyMap{
			"Values": {
				mp(GeoPoint{Lat: 1, Lng: 2}), mp(GeoPoint{Lat: 3, Lng: 4})},
		},
	},
	{
		desc:    "convertable complex slice (bad load)",
		src:     PropertyMap{"Values": {mp("hello")}},
		want:    &Impossible4{[]Complex(nil)},
		loadErr: "nope",
	},
	{
		desc: "allow concrete *Key implementors (save)",
		src:  &DerivedKey{testKey2a},
		want: &IfaceKey{testKey2b},
	},
	{
		desc: "allow concrete *Key implementors (load)",
		src:  &IfaceKey{testKey2b},
		want: &DerivedKey{testKey2a},
	},
	{
		desc:    "save []float64 load []int64",
		src:     &Y0{B: true, F: []float64{7, 8, 9}},
		want:    &Y2{B: true},
		loadErr: "type mismatch",
	},
	{
		desc:    "single slice is too long",
		src:     &Y0{F: make([]float64, maxIndexedProperties+1)},
		want:    &Y0{},
		saveErr: "gae: too many indexed properties",
	},
	{
		desc:    "two slices are too long",
		src:     &Y0{F: make([]float64, maxIndexedProperties), G: make([]float64, maxIndexedProperties)},
		want:    &Y0{},
		saveErr: "gae: too many indexed properties",
	},
	{
		desc:    "one slice and one scalar are too long",
		src:     &Y0{F: make([]float64, maxIndexedProperties), B: true},
		want:    &Y0{},
		saveErr: "gae: too many indexed properties",
	},
	{
		desc: "long blob",
		src:  &B0{B: makeUint8Slice(maxIndexedProperties + 1)},
		want: &B0{B: makeUint8Slice(maxIndexedProperties + 1)},
	},
	{
		desc:    "long []int8 is too long",
		src:     &B1{B: makeInt8Slice(maxIndexedProperties + 1)},
		want:    &B1{},
		saveErr: "gae: too many indexed properties",
	},
	{
		desc: "short []int8",
		src:  &B1{B: makeInt8Slice(3)},
		want: &B1{B: makeInt8Slice(3)},
	},
	{
		desc: "long myBlob",
		src:  &B2{B: makeUint8Slice(maxIndexedProperties + 1)},
		want: &B2{B: makeUint8Slice(maxIndexedProperties + 1)},
	},
	{
		desc: "short myBlob",
		src:  &B2{B: makeUint8Slice(3)},
		want: &B2{B: makeUint8Slice(3)},
	},
	{
		desc: "long []myByte",
		src:  &B3{B: makeMyByteSlice(maxIndexedProperties + 1)},
		want: &B3{B: makeMyByteSlice(maxIndexedProperties + 1)},
	},
	{
		desc: "short []myByte",
		src:  &B3{B: makeMyByteSlice(3)},
		want: &B3{B: makeMyByteSlice(3)},
	},
	{
		desc: "slice of blobs",
		src: &B4{B: [][]byte{
			makeUint8Slice(3),
			makeUint8Slice(4),
			makeUint8Slice(5),
		}},
		want: &B4{B: [][]byte{
			makeUint8Slice(3),
			makeUint8Slice(4),
			makeUint8Slice(5),
		}},
	},
	{
		desc: "short []byte",
		src:  &B5{B: makeUint8Slice(3)},
		want: &B5{B: makeUint8Slice(3)},
	},
	{
		desc: "short ByteString as props",
		src:  &B5{B: makeUint8Slice(3)},
		want: PropertyMap{
			"B": {mp(makeUint8Slice(3))},
		},
	},
	{
		desc: "save tagged load props",
		src:  &Tagged{A: 1, B: []int{21, 22, 23}, C: 3, D: 4, E: 5, I: 6, J: 7},
		want: PropertyMap{
			// A and B are renamed to a and b; A and C are noindex, I is ignored.
			// Indexed properties are loaded before raw properties. Thus, the
			// result is: b, b, b, D, E, a, c.
			"b1": {
				mp(21),
				mp(22),
				mp(23),
			},
			"D": {mp(4)},
			"E": {mp(5)},
			"a": {mpNI(1)},
			"C": {mpNI(3)},
			"J": {mpNI(7)},
		},
	},
	{
		desc: "save tagged load tagged",
		src:  &Tagged{A: 1, B: []int{21, 22, 23}, C: 3, D: 4, E: 5, I: 6, J: 7},
		want: &Tagged{A: 1, B: []int{21, 22, 23}, C: 3, D: 4, E: 5, J: 7},
	},
	{
		desc: "save props load tagged",
		src: PropertyMap{
			"A": {mpNI(11)},
			"a": {mpNI(12)},
		},
		want:    &Tagged{A: 12},
		loadErr: `cannot load field "A"`,
	},
	{
		desc:   "invalid tagged1",
		src:    &InvalidTagged1{I: 1},
		plsErr: `struct tag has invalid property name: "\t"`,
	},
	{
		desc:   "invalid tagged2",
		src:    &InvalidTagged2{I: 1, J: 2},
		want:   &InvalidTagged2{},
		plsErr: `struct tag has repeated property name: "I"`,
	},
	{
		desc:   "invalid tagged3",
		src:    &InvalidTagged3{I: 1},
		plsErr: `struct tag has invalid property name: "a\t"`,
	},
	{
		desc:   "invalid tagged4",
		src:    &InvalidTagged4{I: 1},
		plsErr: `struct tag has invalid property name: "a."`,
	},
	{
		desc:   "invalid tagged5",
		src:    &InvalidTagged5{I: 19, V: []InvalidTaggedSub{{1}}},
		plsErr: `struct tag has repeated property name: "V.I"`,
	},
	{
		desc: "doubler",
		src:  &Doubler{S: "s", I: 1, B: true},
		want: &Doubler{S: "ss", I: 2, B: true},
	},
	{
		desc: "save struct load props",
		src:  &X0{S: "s", I: 1},
		want: PropertyMap{
			"S": {mp("s")},
			"I": {mp(1)},
		},
	},
	{
		desc: "save props load struct",
		src: PropertyMap{
			"S": {mp("s")},
			"I": {mp(1)},
		},
		want: &X0{S: "s", I: 1},
	},
	{
		desc: "nil-value props",
		src: PropertyMap{
			"I": {mp(nil)},
			"B": {mp(nil)},
			"S": {mp(nil)},
			"F": {mp(nil)},
			"K": {mp(nil)},
			"T": {mp(nil)},
			"J": {
				mp(nil),
				mp(7),
				mp(nil),
			},
		},
		want: &struct {
			I int64
			B bool
			S string
			F float64
			K *Key
			T time.Time
			J []int64
		}{
			J: []int64{0, 7, 0},
		},
	},
	{
		desc: "save outer load props",
		src: &Outer{
			A: 1,
			I: []Inner1{
				{10, "ten"},
				{20, "twenty"},
				{30, "thirty"},
			},
			J: Inner2{
				Y: 3.14,
			},
			Inner3: Inner3{
				Z: true,
			},
		},
		want: PropertyMap{
			"A": {mp(1)},
			"I.W": {
				mp(10),
				mp(20),
				mp(30),
			},
			"I.X": {
				mp("ten"),
				mp("twenty"),
				mp("thirty"),
			},
			"J.Y": {mp(3.14)},
			"Z":   {mp(true)},
		},
	},
	{
		desc: "save props load outer-equivalent",
		src: PropertyMap{
			"A": {mp(1)},
			"I.W": {
				mp(10),
				mp(20),
				mp(30),
			},
			"I.X": {
				mp("ten"),
				mp("twenty"),
				mp("thirty"),
			},
			"J.Y": {mp(3.14)},
			"Z":   {mp(true)},
		},
		want: &OuterEquivalent{
			A:     1,
			IDotW: []int32{10, 20, 30},
			IDotX: []string{"ten", "twenty", "thirty"},
			JDotY: 3.14,
			Z:     true,
		},
	},
	{
		desc: "save outer-equivalent load outer",
		src: &OuterEquivalent{
			A:     1,
			IDotW: []int32{10, 20, 30},
			IDotX: []string{"ten", "twenty", "thirty"},
			JDotY: 3.14,
			Z:     true,
		},
		want: &Outer{
			A: 1,
			I: []Inner1{
				{10, "ten"},
				{20, "twenty"},
				{30, "thirty"},
			},
			J: Inner2{
				Y: 3.14,
			},
			Inner3: Inner3{
				Z: true,
			},
		},
	},
	{
		desc: "dotted names save",
		src:  &Dotted{A: DottedA{B: DottedB{C: 88}}},
		want: PropertyMap{
			"A0.A1.A2.B3.C4.C5": {mp(88)},
		},
	},
	{
		desc: "dotted names load",
		src: PropertyMap{
			"A0.A1.A2.B3.C4.C5": {mp(99)},
		},
		want: &Dotted{A: DottedA{B: DottedB{C: 99}}},
	},
	{
		desc: "save struct load deriver",
		src:  &X0{S: "s", I: 1},
		want: &Deriver{S: "s", Derived: "derived+s"},
	},
	{
		desc: "save deriver load struct",
		src:  &Deriver{S: "s", Derived: "derived+s", Ignored: "ignored"},
		want: &X0{S: "s"},
	},
	{
		desc: "augmenter save",
		src:  &Augmenter{S: "s"},
		want: PropertyMap{
			"S":     {mp("s")},
			"Extra": {mp("ohai!")},
		},
	},
	{
		desc: "augmenter load",
		src: PropertyMap{
			"S":     {mp("s")},
			"Extra": {mp("kthxbye!")},
		},
		want: &Augmenter{S: "s", g: "kthxbye!"},
	},
	// Regression: CL 25062824 broke handling of appengine.BlobKey fields.
	{
		desc: "appengine.BlobKey",
		src:  &BK{Key: "blah"},
		want: &BK{Key: "blah"},
	},
	{
		desc: "zero time.Time",
		src:  &T{T: time.Time{}},
		want: &T{T: time.Time{}},
	},
	{
		desc: "time.Time near Unix zero time",
		src:  &T{T: time.Unix(0, 4e3).UTC()},
		want: &T{T: time.Unix(0, 4e3).UTC()},
	},
	{
		desc: "time.Time, far in the future",
		src:  &T{T: time.Date(99999, 1, 1, 0, 0, 0, 0, time.UTC)},
		want: &T{T: time.Date(99999, 1, 1, 0, 0, 0, 0, time.UTC)},
	},
	{
		desc:    "time.Time, very far in the past",
		src:     &T{T: time.Date(-300000, 1, 1, 0, 0, 0, 0, time.UTC)},
		want:    &T{},
		saveErr: "time value out of range",
	},
	{
		desc:    "time.Time, very far in the future",
		src:     &T{T: time.Date(294248, 1, 1, 0, 0, 0, 0, time.UTC)},
		want:    &T{},
		saveErr: "time value out of range",
	},
	{
		desc: "structs",
		src: &N0{
			X0:       X0{S: "one", I: 2, i: 3},
			Nonymous: X0{S: "four", I: 5, i: 6},
			Ignore:   "ignore",
			Other:    "other",
		},
		want: &N0{
			X0:       X0{S: "one", I: 2},
			Nonymous: X0{S: "four", I: 5},
			Other:    "other",
		},
	},
	{
		desc: "exotic types",
		src: &ExoticTypes{
			BS: "sup",
		},
		want: &ExoticTypes{
			BS: "sup",
		},
	},
	{
		desc:   "underspecified types",
		src:    &Underspecified{},
		plsErr: "non-concrete interface",
	},
	{
		desc: "mismatch (string)",
		src: PropertyMap{
			"K": {mp(199)},
			"S": {mp([]byte("cats"))},
			"F": {mp("nurbs")},
		},
		want:    &MismatchTypes{},
		loadErr: "type mismatch",
	},
	{
		desc:    "mismatch (float)",
		src:     PropertyMap{"F": {mp(blobstore.Key("wot"))}},
		want:    &MismatchTypes{},
		loadErr: "type mismatch",
	},
	{
		desc:    "mismatch (float/overflow)",
		src:     PropertyMap{"F": {mp(math.MaxFloat64)}},
		want:    &MismatchTypes{},
		loadErr: "overflows",
	},
	{
		desc:    "mismatch (key)",
		src:     PropertyMap{"K": {mp(false)}},
		want:    &MismatchTypes{},
		loadErr: "type mismatch",
	},
	{
		desc:    "mismatch (bool)",
		src:     PropertyMap{"B": {mp(testKey0)}},
		want:    &MismatchTypes{},
		loadErr: "type mismatch",
	},
	{
		desc:    "mismatch (time)",
		src:     PropertyMap{"T": {mp(GeoPoint{})}},
		want:    &MismatchTypes{},
		loadErr: "type mismatch",
	},
	{
		desc:    "mismatch (geopoint)",
		src:     PropertyMap{"G": {mp(time.Now().UTC())}},
		want:    &MismatchTypes{},
		loadErr: "type mismatch",
	},
	{
		desc: "slice of structs",
		src: &N1{
			X0: X0{S: "one", I: 2, i: 3},
			Nonymous: []X0{
				{S: "four", I: 5, i: 6},
				{S: "seven", I: 8, i: 9},
				{S: "ten", I: 11, i: 12},
				{S: "thirteen", I: 14, i: 15},
			},
			Ignore: "ignore",
			Other:  "other",
		},
		want: &N1{
			X0: X0{S: "one", I: 2},
			Nonymous: []X0{
				{S: "four", I: 5},
				{S: "seven", I: 8},
				{S: "ten", I: 11},
				{S: "thirteen", I: 14},
			},
			Other: "other",
		},
	},
	{
		desc: "structs with slices of structs",
		src: &N2{
			N1: N1{
				X0: X0{S: "rouge"},
				Nonymous: []X0{
					{S: "rosso0"},
					{S: "rosso1"},
				},
			},
			Green: N1{
				X0: X0{S: "vert"},
				Nonymous: []X0{
					{S: "verde0"},
					{S: "verde1"},
					{S: "verde2"},
				},
			},
			Blue: N1{
				X0: X0{S: "bleu"},
				Nonymous: []X0{
					{S: "blu0"},
					{S: "blu1"},
					{S: "blu2"},
					{S: "blu3"},
				},
			},
		},
		want: &N2{
			N1: N1{
				X0: X0{S: "rouge"},
				Nonymous: []X0{
					{S: "rosso0"},
					{S: "rosso1"},
				},
			},
			Green: N1{
				X0: X0{S: "vert"},
				Nonymous: []X0{
					{S: "verde0"},
					{S: "verde1"},
					{S: "verde2"},
				},
			},
			Blue: N1{
				X0: X0{S: "bleu"},
				Nonymous: []X0{
					{S: "blu0"},
					{S: "blu1"},
					{S: "blu2"},
					{S: "blu3"},
				},
			},
		},
	},
	{
		desc: "save structs load props",
		src: &N2{
			N1: N1{
				X0: X0{S: "rouge"},
				Nonymous: []X0{
					{S: "rosso0"},
					{S: "rosso1"},
				},
			},
			Green: N1{
				X0: X0{S: "vert"},
				Nonymous: []X0{
					{S: "verde0"},
					{S: "verde1"},
					{S: "verde2"},
				},
			},
			Blue: N1{
				X0: X0{S: "bleu"},
				Nonymous: []X0{
					{S: "blu0"},
					{S: "blu1"},
					{S: "blu2"},
					{S: "blu3"},
				},
			},
		},
		want: PropertyMap{
			"red.S":            {mp("rouge")},
			"red.I":            {mp(0)},
			"red.Nonymous.S":   {mp("rosso0"), mp("rosso1")},
			"red.Nonymous.I":   {mp(0), mp(0)},
			"red.Other":        {mp("")},
			"green.S":          {mp("vert")},
			"green.I":          {mp(0)},
			"green.Nonymous.S": {mp("verde0"), mp("verde1"), mp("verde2")},
			"green.Nonymous.I": {mp(0), mp(0), mp(0)},
			"green.Other":      {mp("")},
			"Blue.S":           {mp("bleu")},
			"Blue.I":           {mp(0)},
			"Blue.Nonymous.S":  {mp("blu0"), mp("blu1"), mp("blu2"), mp("blu3")},
			"Blue.Nonymous.I":  {mp(0), mp(0), mp(0), mp(0)},
			"Blue.Other":       {mp("")},
		},
	},
	{
		desc: "save props load structs with ragged fields",
		src: PropertyMap{
			"red.S":            {mp("rot")},
			"green.Nonymous.I": {mp(10), mp(11), mp(12), mp(13)},
			"Blue.Nonymous.S":  {mp("blau0"), mp("blau1"), mp("blau2")},
			"Blue.Nonymous.I":  {mp(20), mp(21)},
		},
		want: &N2{
			N1: N1{
				X0: X0{S: "rot"},
			},
			Green: N1{
				Nonymous: []X0{
					{I: 10},
					{I: 11},
					{I: 12},
					{I: 13},
				},
			},
			Blue: N1{
				Nonymous: []X0{
					{S: "blau0", I: 20},
					{S: "blau1", I: 21},
					{S: "blau2"},
				},
			},
		},
	},
	{
		desc: "save structs with noindex tags",
		src: &struct {
			A struct {
				X string `gae:",noindex"`
				Y string
			} `gae:",noindex"`
			B struct {
				X string `gae:",noindex"`
				Y string
			}
		}{},
		want: PropertyMap{
			"B.Y": {mp("")},
			"A.X": {mpNI("")},
			"A.Y": {mpNI("")},
			"B.X": {mpNI("")},
		},
	},
	{
		desc: "embedded struct with name override",
		src: &struct {
			Inner1 `gae:"foo"`
		}{},
		want: PropertyMap{
			"foo.W": {mp(0)},
			"foo.X": {mp("")},
		},
	},
	{
		desc:   "slice of slices",
		src:    &SliceOfSlices{},
		plsErr: `flattening nested structs leads to a slice of slices: field "S"`,
	},
	{
		desc:   "recursive struct",
		src:    &Recursive{},
		plsErr: `field "R" is recursively defined`,
	},
	{
		desc:   "mutually recursive struct",
		src:    &MutuallyRecursive0{},
		plsErr: `field "R" has problem: field "R" is recursively defined`,
	},
	{
		desc: "non-exported struct fields",
		src: &struct {
			i, J int64
		}{i: 1, J: 2},
		want: PropertyMap{
			"J": {mp(2)},
		},
	},
	{
		desc: "json.RawMessage",
		src: &struct {
			J json.RawMessage
		}{
			J: json.RawMessage("rawr"),
		},
		want: PropertyMap{
			"J": {mp([]byte("rawr"))},
		},
	},
	{
		desc: "json.RawMessage to myBlob",
		src: &struct {
			B json.RawMessage
		}{
			B: json.RawMessage("rawr"),
		},
		want: &B2{B: myBlob("rawr")},
	},
}

// checkErr returns the empty string if either both want and err are zero,
// or if want is a non-empty substring of err's string representation.
func checkErr(want string, err error) string {
	if err != nil {
		got := err.Error()
		if want == "" || strings.Index(got, want) == -1 {
			return got
		}
	} else if want != "" {
		return fmt.Sprintf("want error %q", want)
	}
	return ""
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	checkErr := func(actual interface{}, expected string) bool {
		if expected == "" {
			So(actual, ShouldErrLike, nil)
		} else {
			So(actual, ShouldErrLike, expected)
		}
		return expected != ""
	}

	Convey("Test round-trip", t, func() {
		for _, tc := range testCases {
			tc := tc
			Convey(tc.desc, func() {
				pls, ok := tc.src.(PropertyLoadSaver)
				if !ok {
					pls = GetPLS(tc.src)
				}
				if checkErr(pls.Problem(), tc.plsErr) {
					return
				}
				So(pls, ShouldNotBeNil)

				savedProps, err := pls.Save(false)
				if checkErr(err, tc.saveErr) {
					return
				}
				So(savedProps, ShouldNotBeNil)

				var got interface{}
				if _, ok := tc.want.(PropertyMap); ok {
					pls = PropertyMap{}
					got = pls
				} else {
					got = reflect.New(reflect.TypeOf(tc.want).Elem()).Interface()
					if pls, ok = got.(PropertyLoadSaver); !ok {
						pls = GetPLS(got)
					}
				}

				if checkErr(pls.Problem(), tc.plsLoadErr) {
					return
				}
				So(pls, ShouldNotBeNil)

				err = pls.Load(savedProps)
				if checkErr(err, tc.loadErr) {
					return
				}
				if tc.want == nil {
					return
				}

				if gotT, ok := got.(*T); ok {
					// Round tripping a time.Time can result in a different time.Location: Local instead of UTC.
					// We therefore test equality explicitly, instead of relying on reflect.DeepEqual.
					So(gotT.T.Equal(tc.want.(*T).T), ShouldBeTrue)
				} else {
					So(got, ShouldResemble, tc.want)
				}
			})
		}
	})
}

func TestMeta(t *testing.T) {
	t.Parallel()

	Convey("Test meta fields", t, func() {
		Convey("Can retrieve from struct", func() {
			o := &N0{ID: 100}
			mgs := getMGS(o)
			val, err := mgs.GetMeta("id")
			So(err, ShouldBeNil)
			So(val, ShouldEqual, 100)

			val, err = mgs.GetMeta("kind")
			So(err, ShouldBeNil)
			So(val, ShouldEqual, "whatnow")

			So(mgs.GetMetaDefault("kind", "zappo"), ShouldEqual, "whatnow")
			So(mgs.GetMetaDefault("id", "stringID"), ShouldEqual, "stringID")
			So(mgs.GetMetaDefault("id", 6), ShouldEqual, 100)
		})

		Convey("Getting something not there is an error", func() {
			o := &N0{ID: 100}
			mgs := getMGS(o)
			_, err := mgs.GetMeta("wat")
			So(err, ShouldEqual, ErrMetaFieldUnset)
		})

		Convey("Default works for missing fields", func() {
			o := &N0{ID: 100}
			mgs := getMGS(o)
			So(mgs.GetMetaDefault("whozit", 10), ShouldEqual, 10)
		})

		Convey("getting/setting from a bad struct is an error", func() {
			o := &Recursive{}
			mgs := getMGS(o)
			_, err := mgs.GetMeta("wat")
			So(err, ShouldNotBeNil)

			err = mgs.SetMeta("wat", 100)
			So(err, ShouldNotBeNil)
		})

		Convey("Default works for bad structs", func() {
			o := &Recursive{}
			mgs := getMGS(o)
			So(mgs.GetMetaDefault("whozit", 10), ShouldEqual, 10)
		})

		Convey("can assign values to exported meta fields", func() {
			o := &N0{ID: 100}
			mgs := getMGS(o)
			err := mgs.SetMeta("id", int64(200))
			So(err, ShouldBeNil)
			So(o.ID, ShouldEqual, 200)

		})

		Convey("assigning to unsassiagnable fields is a simple error", func() {
			o := &N0{ID: 100}
			mgs := getMGS(o)
			err := mgs.SetMeta("kind", "hi")
			So(err.Error(), ShouldContainSubstring, "unexported field")

			err = mgs.SetMeta("noob", "hi")
			So(err, ShouldEqual, ErrMetaFieldUnset)
		})
	})

	Convey("StructPLS Miscellaneous", t, func() {
		Convey("a simple struct has a default $kind", func() {
			So(GetPLS(&Simple{}).GetAllMeta(), ShouldResemble, PropertyMap{
				"$kind": []Property{mpNI("Simple")},
			})
		})

		Convey("multiple overlapping fields is an error", func() {
			o := &BadMeta{}
			pls := GetPLS(o)
			err := pls.Load(nil)
			So(err, ShouldErrLike, "multiple times")
			e := pls.Problem()
			_, err = pls.Save(true)
			So(err, ShouldEqual, e)
			err = pls.Load(nil)
			So(err, ShouldEqual, e)
		})

		Convey("empty property names are invalid", func() {
			So(validPropertyName(""), ShouldBeFalse)
		})

		Convey("attempting to get a PLS for a non *struct is an error", func() {
			pls := GetPLS((*[]string)(nil))
			So(pls.Problem(), ShouldEqual, ErrInvalidEntityType)

			Convey("the error PLS can still be used", func() {
				k, _ := pls.GetMeta("kind")
				So(k, ShouldBeNil)

				props := pls.GetAllMeta()
				So(props, ShouldResemble, PropertyMap{
					"$kind": []Property{mpNI("")},
				})

				_, err := pls.Save(true)
				So(err, ShouldNotBeNil)
			})
		})

		Convey("convertible meta default types", func() {
			type OKDefaults struct {
				When   string `gae:"$when,tomorrow"`
				Amount int64  `gae:"$amt,100"`
				DoIt   Toggle `gae:"$doit,on"`
			}
			okd := &OKDefaults{}
			pls := GetPLS(okd)
			mgs := getMGS(okd)
			So(pls.Problem(), ShouldBeNil)

			v, err := mgs.GetMeta("when")
			So(err, ShouldBeNil)
			So(v, ShouldEqual, "tomorrow")

			v, err = mgs.GetMeta("amt")
			So(err, ShouldBeNil)
			So(v, ShouldEqual, int64(100))

			So(okd.DoIt, ShouldEqual, Auto)
			v, err = mgs.GetMeta("doit")
			So(err, ShouldBeNil)
			So(v, ShouldBeTrue)

			err = mgs.SetMeta("doit", false)
			So(err, ShouldBeNil)
			v, err = mgs.GetMeta("doit")
			So(err, ShouldBeNil)
			So(v, ShouldBeFalse)
			So(okd.DoIt, ShouldEqual, Off)

			err = mgs.SetMeta("doit", true)
			So(err, ShouldBeNil)
			v, err = mgs.GetMeta("doit")
			So(err, ShouldBeNil)
			So(v, ShouldBeTrue)
			So(okd.DoIt, ShouldEqual, On)

			Convey("Toggle fields REQUIRE a default", func() {
				type BadToggle struct {
					Bad Toggle `gae:"$wut"`
				}
				pls := GetPLS(&BadToggle{})
				So(pls.Problem().Error(), ShouldContainSubstring, "bad/missing default")
			})
		})

		Convey("meta fields can be saved", func() {
			type OKDefaults struct {
				When   string `gae:"$when,tomorrow"`
				Amount int64  `gae:"$amt,100"`
			}
			pls := GetPLS(&OKDefaults{})
			pm, err := pls.Save(true)
			So(err, ShouldBeNil)
			So(pm, ShouldResemble, PropertyMap{
				"$when": {mpNI("tomorrow")},
				"$amt":  {mpNI(100)},
				"$kind": {mpNI("OKDefaults")},
			})

			v, err := pm.GetMeta("when")
			So(err, ShouldBeNil)
			So(v, ShouldEqual, "tomorrow")

			v, err = pm.GetMeta("amt")
			So(err, ShouldBeNil)
			So(v, ShouldEqual, int64(100))
		})

		Convey("default are optional", func() {
			type OverrideDefault struct {
				Val int64 `gae:"$val"`
			}
			o := &OverrideDefault{}
			mgs := getMGS(o)

			v, err := mgs.GetMeta("val")
			So(err, ShouldBeNil)
			So(v, ShouldEqual, int64(0))
		})

		Convey("overridable defaults", func() {
			type OverrideDefault struct {
				Val int64 `gae:"$val,100"`
			}
			o := &OverrideDefault{}
			mgs := getMGS(o)

			v, err := mgs.GetMeta("val")
			So(err, ShouldBeNil)
			So(v, ShouldEqual, int64(100))

			o.Val = 10
			v, err = mgs.GetMeta("val")
			So(err, ShouldBeNil)
			So(v, ShouldEqual, int64(10))
		})

		Convey("Bad default meta type", func() {
			type BadDefault struct {
				Val time.Time `gae:"$meta,tomorrow"`
			}
			pls := GetPLS(&BadDefault{})
			So(pls.Problem().Error(), ShouldContainSubstring, "bad type")
		})

		Convey("MetaGetterSetter implementation (IDParser)", func() {
			idp := &IDParser{parent: "moo", id: 100}
			mgs := getMGS(idp)
			So(mgs.GetMetaDefault("id", ""), ShouldEqual, "moo|100")
			So(mgs.GetMetaDefault("kind", ""), ShouldEqual, "CoolKind")

			So(mgs.SetMeta("kind", "Something"), ShouldErrLike, "unexported field")
			So(mgs.SetMeta("id", "happy|27"), ShouldBeNil)

			So(idp.parent, ShouldEqual, "happy")
			So(idp.id, ShouldEqual, 27)

			So(mgs.GetAllMeta(), ShouldResemble, PropertyMap{
				"$id":   {mpNI("happy|27")},
				"$kind": {mpNI("CoolKind")},
			})
		})

		Convey("MetaGetterSetter implementation (KindOverride)", func() {
			ko := &KindOverride{ID: 20}
			mgs := getMGS(ko)
			So(mgs.GetMetaDefault("kind", ""), ShouldEqual, "KindOverride")

			ko.customKind = "something"
			So(mgs.GetMetaDefault("kind", ""), ShouldEqual, "something")

			So(mgs.SetMeta("kind", "Nerp"), ShouldBeNil)
			So(ko.customKind, ShouldEqual, "Nerp")

			So(mgs.SetMeta("kind", "KindOverride"), ShouldBeNil)
			So(ko.customKind, ShouldEqual, "")

			So(mgs.GetAllMeta(), ShouldResemble, PropertyMap{
				"$id":   {mpNI(20)},
				"$kind": {mpNI("KindOverride")},
			})
			ko.customKind = "wut"
			So(mgs.GetAllMeta(), ShouldResemble, PropertyMap{
				"$id":   {mpNI(20)},
				"$kind": {mpNI("wut")},
			})

			props, err := GetPLS(ko).Save(true)
			So(err, ShouldBeNil)
			So(props, ShouldResemble, PropertyMap{
				"$id":   {mpNI(20)},
				"$kind": {mpNI("wut")},
			})
		})

		Convey("Embeddable Metadata structs", func() {
			ide := &IDEmbedder{EmbeddedID{"hello", 10}}
			pls := GetPLS(ide)
			val, err := pls.GetMeta("id")
			So(err, ShouldBeNil)
			So(val, ShouldEqual, "hello|10")

			So(pls.SetMeta("id", "sup|1337"), ShouldBeNil)
			So(ide.EmbeddedID, ShouldResemble, EmbeddedID{"sup", 1337})

			So(pls.GetAllMeta(), ShouldResembleV, PropertyMap{
				"$id":   {mpNI("sup|1337")},
				"$kind": {mpNI("IDEmbedder")},
			})
		})
	})
}
