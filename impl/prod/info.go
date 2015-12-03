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

type key int

var probeCacheKey key

// useGI adds a gae.GlobalInfo implementation to context, accessible
// by gae.GetGI(c)
func useGI(c context.Context) context.Context {
	probeCache := getProbeCache(c)
	if probeCache == nil {
		c = withProbeCache(c, probe(c))
	}

	return info.SetFactory(c, func(ci context.Context) info.Interface {
		return giImpl{ci}
	})
}

type giImpl struct{ context.Context }

func (g giImpl) AccessToken(scopes ...string) (token string, expiry time.Time, err error) {
	return appengine.AccessToken(g, scopes...)
}
func (g giImpl) AppID() string {
	return appengine.AppID(g)
}
func (g giImpl) FullyQualifiedAppID() string {
	return getProbeCache(g).fqaid
}
func (g giImpl) GetNamespace() string {
	return getProbeCache(g).namespace
}
func (g giImpl) Datacenter() string {
	return appengine.Datacenter(g)
}
func (g giImpl) DefaultVersionHostname() string {
	return appengine.DefaultVersionHostname(g)
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
	return appengine.ModuleHostname(g, module, version, instance)
}
func (g giImpl) ModuleName() (name string) {
	return appengine.ModuleName(g)
}
func (g giImpl) Namespace(namespace string) (context.Context, error) {
	c, err := appengine.Namespace(g, namespace)
	if err != nil {
		return c, err
	}
	pc := *getProbeCache(g)
	pc.namespace = namespace
	return withProbeCache(c, &pc), nil
}
func (g giImpl) PublicCertificates() ([]info.Certificate, error) {
	certs, err := appengine.PublicCertificates(g)
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
	return appengine.RequestID(g)
}
func (g giImpl) ServerSoftware() string {
	return appengine.ServerSoftware()
}
func (g giImpl) ServiceAccount() (string, error) {
	return appengine.ServiceAccount(g)
}
func (g giImpl) SignBytes(bytes []byte) (keyName string, signature []byte, err error) {
	return appengine.SignBytes(g, bytes)
}
func (g giImpl) VersionID() string {
	return appengine.VersionID(g)
}

type infoProbeCache struct {
	namespace string
	fqaid     string
}

func probe(c context.Context) *infoProbeCache {
	probeKey := datastore.NewKey(c, "Kind", "id", 0, nil)
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
