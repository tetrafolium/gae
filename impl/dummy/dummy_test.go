// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dummy

import (
	"testing"

	dsS "github.com/tetrafolium/gae/service/datastore"
	infoS "github.com/tetrafolium/gae/service/info"
	mailS "github.com/tetrafolium/gae/service/mail"
	mcS "github.com/tetrafolium/gae/service/memcache"
	modS "github.com/tetrafolium/gae/service/module"
	tqS "github.com/tetrafolium/gae/service/taskqueue"
	userS "github.com/tetrafolium/gae/service/user"
	. "github.com/smartystreets/goconvey/convey"
	"golang.org/x/net/context"
)

func TestContextAccess(t *testing.T) {
	t.Parallel()

	// p is a function which recovers an error and then immediately panics with
	// the contained string. It's defer'd in each test so that we can use the
	// ShouldPanicWith assertion (which does an == comparison and not
	// a reflect.DeepEquals comparison).
	p := func() { panic(recover().(error).Error()) }

	Convey("Context Access", t, func() {
		c := context.Background()

		Convey("blank", func() {
			So(dsS.GetRaw(c), ShouldBeNil)
			So(mcS.GetRaw(c), ShouldBeNil)
			So(tqS.GetRaw(c), ShouldBeNil)
			So(infoS.Get(c), ShouldBeNil)
		})

		// needed for everything else
		c = infoS.Set(c, Info())

		Convey("Info", func() {
			So(infoS.Get(c), ShouldNotBeNil)
			So(func() {
				defer p()
				infoS.Get(c).Datacenter()
			}, ShouldPanicWith, "dummy: method Info.Datacenter is not implemented")
		})

		Convey("Datastore", func() {
			c = dsS.SetRaw(c, Datastore())
			So(dsS.Get(c), ShouldNotBeNil)
			So(func() {
				defer p()
				_, _ = dsS.Get(c).DecodeCursor("wut")
			}, ShouldPanicWith, "dummy: method Datastore.DecodeCursor is not implemented")
		})

		Convey("Memcache", func() {
			c = mcS.SetRaw(c, Memcache())
			So(mcS.Get(c), ShouldNotBeNil)
			So(func() {
				defer p()
				_ = mcS.Get(c).Add(nil)
			}, ShouldPanicWith, "dummy: method Memcache.AddMulti is not implemented")
		})

		Convey("TaskQueue", func() {
			c = tqS.SetRaw(c, TaskQueue())
			So(tqS.Get(c), ShouldNotBeNil)
			So(func() {
				defer p()
				_ = tqS.Get(c).Purge("")
			}, ShouldPanicWith, "dummy: method TaskQueue.Purge is not implemented")
		})

		Convey("User", func() {
			c = userS.Set(c, User())
			So(userS.Get(c), ShouldNotBeNil)
			So(func() {
				defer p()
				_ = userS.Get(c).IsAdmin()
			}, ShouldPanicWith, "dummy: method User.IsAdmin is not implemented")
		})

		Convey("Mail", func() {
			c = mailS.Set(c, Mail())
			So(mailS.Get(c), ShouldNotBeNil)
			So(func() {
				defer p()
				_ = mailS.Get(c).Send(nil)
			}, ShouldPanicWith, "dummy: method Mail.Send is not implemented")
		})

		Convey("Module", func() {
			c = modS.Set(c, Module())
			So(modS.Get(c), ShouldNotBeNil)
			So(func() {
				defer p()
				modS.Get(c).List()
			}, ShouldPanicWith, "dummy: method Module.List is not implemented")
		})
	})
}
