// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"fmt"
	"regexp"

	"github.com/tetrafolium/gae/impl/dummy"
	"github.com/tetrafolium/gae/service/info"
	"golang.org/x/net/context"
)

type giContextKeyType int

var giContextKey giContextKeyType

// validNamespace matches valid namespace names.
var validNamespace = regexp.MustCompile(`^[0-9A-Za-z._-]{0,100}$`)

func curGID(c context.Context) *globalInfoData {
	return c.Value(giContextKey).(*globalInfoData)
}

// useGI adds a gae.GlobalInfo context, accessible
// by gae.GetGI(c)
func useGI(c context.Context, appID string) context.Context {
	return info.SetFactory(c, func(ic context.Context) info.Interface {
		return &giImpl{dummy.Info(), curGID(ic), ic}
	})
}

type globalInfoData struct {
	appid     string
	namespace string
}

type giImpl struct {
	info.Interface
	*globalInfoData
	c context.Context
}

var _ = info.Interface((*giImpl)(nil))

func (gi *giImpl) GetNamespace() string {
	return gi.namespace
}

func (gi *giImpl) Namespace(ns string) (ret context.Context, err error) {
	if !validNamespace.MatchString(ns) {
		return nil, fmt.Errorf("appengine: namespace %q does not match /%s/", ns, validNamespace)
	}
	return context.WithValue(gi.c, giContextKey, &globalInfoData{gi.appid, ns}), nil
}

func (gi *giImpl) MustNamespace(ns string) context.Context {
	ret, err := gi.Namespace(ns)
	if err != nil {
		panic(err)
	}
	return ret
}

func (gi *giImpl) AppID() string {
	return gi.appid
}

func (gi *giImpl) FullyQualifiedAppID() string {
	return gi.appid
}

func (gi *giImpl) IsDevAppServer() bool {
	return true
}

func (gi *giImpl) VersionID() string {
	// VersionID returns X.Y where Y is autogenerated by appengine, and X is
	// whatever's in app.yaml.
	return "testVersionID.1"
}
