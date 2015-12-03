// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// adapted from github.com/golang/appengine/datastore

package datastore

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/tetrafolium/gae/service/info"
	"golang.org/x/net/context"
)

type fakeRDS struct{ RawInterface }

func TestCheckFilter(t *testing.T) {
	t.Parallel()

	Convey("Test checkFilter", t, func() {
		// Note that the way we have this context set up, any calls which aren't
		// stopped at the checkFilter will nil-pointer panic. We use this panic
		// behavior to indicate that the checkfilter has allowed a call to pass
		// through to the implementation in the tests below. In a real application
		// the panics observed in the tests below would actually be sucessful calls
		// to the implementation.
		c := SetRaw(info.Set(context.Background(), fakeInfo{}), fakeRDS{})
		rds := GetRaw(c) // has checkFilter
		So(rds, ShouldNotBeNil)

		Convey("RunInTransaction", func() {
			So(rds.RunInTransaction(nil, nil).Error(), ShouldContainSubstring, "is nil")
			hit := false
			So(func() {
				So(rds.RunInTransaction(func(context.Context) error {
					hit = true
					return nil
				}, nil), ShouldBeNil)
			}, ShouldPanic)
			So(hit, ShouldBeFalse)
		})

		Convey("Run", func() {
			So(rds.Run(nil, nil).Error(), ShouldContainSubstring, "query is nil")
			fq, err := NewQuery("sup").Finalize()
			So(err, ShouldBeNil)

			So(rds.Run(fq, nil).Error(), ShouldContainSubstring, "callback is nil")
			hit := false
			So(func() {
				So(rds.Run(fq, func(*Key, PropertyMap, CursorCB) bool {
					hit = true
					return true
				}), ShouldBeNil)
			}, ShouldPanic)
			So(hit, ShouldBeFalse)
		})

		Convey("GetMulti", func() {
			So(rds.GetMulti(nil, nil, nil), ShouldBeNil)
			So(rds.GetMulti([]*Key{mkKey("", "", "", "")}, nil, nil).Error(), ShouldContainSubstring, "is nil")

			// this is in the wrong aid/ns
			keys := []*Key{MakeKey("wut", "wrong", "Kind", 1)}
			So(rds.GetMulti(keys, nil, func(pm PropertyMap, err error) {
				So(pm, ShouldBeNil)
				So(err, ShouldEqual, ErrInvalidKey)
			}), ShouldBeNil)

			keys[0] = mkKey("Kind", 1)
			hit := false
			So(func() {
				So(rds.GetMulti(keys, nil, func(pm PropertyMap, err error) {
					hit = true
				}), ShouldBeNil)
			}, ShouldPanic)
			So(hit, ShouldBeFalse)
		})

		Convey("PutMulti", func() {
			keys := []*Key{}
			vals := []PropertyMap{{}}
			So(rds.PutMulti(keys, vals, nil).Error(),
				ShouldContainSubstring, "mismatched keys/vals")
			So(rds.PutMulti(nil, nil, nil), ShouldBeNil)

			keys = append(keys, mkKey("aid", "ns", "Wut", 0, "Kind", 0))
			So(rds.PutMulti(keys, vals, nil).Error(), ShouldContainSubstring, "callback is nil")

			So(rds.PutMulti(keys, vals, func(k *Key, err error) {
				So(k, ShouldBeNil)
				So(err, ShouldEqual, ErrInvalidKey)
			}), ShouldBeNil)

			keys = []*Key{mkKey("s~aid", "ns", "Kind", 0)}
			vals = []PropertyMap{nil}
			So(rds.PutMulti(keys, vals, func(k *Key, err error) {
				So(k, ShouldBeNil)
				So(err.Error(), ShouldContainSubstring, "nil vals entry")
			}), ShouldBeNil)

			vals = []PropertyMap{{}}
			hit := false
			So(func() {
				So(rds.PutMulti(keys, vals, func(k *Key, err error) {
					hit = true
				}), ShouldBeNil)
			}, ShouldPanic)
			So(hit, ShouldBeFalse)
		})

		Convey("DeleteMulti", func() {
			So(rds.DeleteMulti(nil, nil), ShouldBeNil)
			So(rds.DeleteMulti([]*Key{mkKey("", "", "", "")}, nil).Error(), ShouldContainSubstring, "is nil")
			So(rds.DeleteMulti([]*Key{mkKey("", "", "", "")}, func(err error) {
				So(err, ShouldEqual, ErrInvalidKey)
			}), ShouldBeNil)

			hit := false
			So(func() {
				So(rds.DeleteMulti([]*Key{mkKey("s~aid", "ns", "Kind", 1)}, func(error) {
					hit = true
				}), ShouldBeNil)
			}, ShouldPanic)
			So(hit, ShouldBeFalse)
		})

	})
}
