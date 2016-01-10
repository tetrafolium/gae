// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package featureBreaker

import (
	"github.com/tetrafolium/gae/service/mail"
	"golang.org/x/net/context"
)

type mailState struct {
	*state

	mail.Interface
}

var _ mail.Interface = (*mailState)(nil)

func (m *mailState) Send(msg *mail.Message) error {
	return m.run(func() error { return m.Interface.Send(msg) })
}

func (m *mailState) SendToAdmins(msg *mail.Message) error {
	return m.run(func() error { return m.Interface.SendToAdmins(msg) })
}

// FilterMail installs a counter mail filter in the context.
func FilterMail(c context.Context, defaultError error) (context.Context, FeatureBreaker) {
	state := newState(defaultError)
	return mail.AddFilters(c, func(ic context.Context, i mail.Interface) mail.Interface {
		return &mailState{state, i}
	}), state
}
