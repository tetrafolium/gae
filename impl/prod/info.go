// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package prod

import (
	"time"

	"github.com/tetrafolium/gae/service/info"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

// useGI adds a gae.GlobalInfo implementation to context, accessible
// by gae.GetGI(c)
func useGI(usrCtx context.Context) context.Context {
	probeCache := getProbeCache(usrCtx)
	if probeCache == nil {
		usrCtx = withProbeCache(usrCtx, probe(AEContext(usrCtx)))
	}

	return info.SetFactory(usrCtx, func(ci context.Context) info.Interface {
		return giImpl{ci, AEContext(ci)}
	})
}

type giImpl struct {
	usrCtx context.Context
	aeCtx  context.Context
}

func (g giImpl) AccessToken(scopes ...string) (token string, expiry time.Time, err error) {
	return appengine.AccessToken(g.aeCtx, scopes...)
}
func (g giImpl) AppID() string {
	return appengine.AppID(g.aeCtx)
}
func (g giImpl) FullyQualifiedAppID() string {
	return getProbeCache(g.usrCtx).fqaid
}
func (g giImpl) GetNamespace() string {
	return getProbeCache(g.usrCtx).namespace
}
func (g giImpl) Datacenter() string {
	return appengine.Datacenter(g.aeCtx)
}
func (g giImpl) DefaultVersionHostname() string {
	return appengine.DefaultVersionHostname(g.aeCtx)
}
func (g giImpl) InstanceID() string {
	return appengine.InstanceID()
}
func (g giImpl) IsDevAppServer() bool {
	return appengine.IsDevAppServer()
}
func (g giImpl) IsOverQuota(err error) bool {
	return appengine.IsOverQuota(err)
}
func (g giImpl) IsTimeoutError(err error) bool {
	return appengine.IsTimeoutError(err)
}
func (g giImpl) ModuleHostname(module, version, instance string) (string, error) {
	return appengine.ModuleHostname(g.aeCtx, module, version, instance)
}
func (g giImpl) ModuleName() (name string) {
	return appengine.ModuleName(g.aeCtx)
}
func (g giImpl) Namespace(namespace string) (context.Context, error) {
	aeCtx, err := appengine.Namespace(g.aeCtx, namespace)
	if err != nil {
		return g.usrCtx, err
	}
	usrCtx := context.WithValue(g.usrCtx, prodContextKey, aeCtx)
	pc := *getProbeCache(usrCtx)
	pc.namespace = namespace
	return withProbeCache(usrCtx, &pc), nil
}
func (g giImpl) MustNamespace(ns string) context.Context {
	ret, err := g.Namespace(ns)
	if err != nil {
		panic(err)
	}
	return ret
}
func (g giImpl) PublicCertificates() ([]info.Certificate, error) {
	certs, err := appengine.PublicCertificates(g.aeCtx)
	if err != nil {
		return nil, err
	}
	ret := make([]info.Certificate, len(certs))
	for i, c := range certs {
		ret[i] = info.Certificate(c)
	}
	return ret, nil
}
func (g giImpl) RequestID() string {
	return appengine.RequestID(g.aeCtx)
}
func (g giImpl) ServerSoftware() string {
	return appengine.ServerSoftware()
}
func (g giImpl) ServiceAccount() (string, error) {
	return appengine.ServiceAccount(g.aeCtx)
}
func (g giImpl) SignBytes(bytes []byte) (keyName string, signature []byte, err error) {
	return appengine.SignBytes(g.aeCtx, bytes)
}
func (g giImpl) VersionID() string {
	return appengine.VersionID(g.aeCtx)
}

type infoProbeCache struct {
	namespace string
	fqaid     string
}

func probe(aeCtx context.Context) *infoProbeCache {
	probeKey := datastore.NewKey(aeCtx, "Kind", "id", 0, nil)
	return &infoProbeCache{
		namespace: probeKey.Namespace(),
		fqaid:     probeKey.AppID(),
	}
}

func getProbeCache(c context.Context) *infoProbeCache {
	if pc, ok := c.Value(probeCacheKey).(*infoProbeCache); ok {
		return pc
	}
	return nil
}

func withProbeCache(c context.Context, pc *infoProbeCache) context.Context {
	return context.WithValue(c, probeCacheKey, pc)
}
