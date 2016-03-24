// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package count

import (
	"github.com/tetrafolium/gae/service/mail"
	"golang.org/x/net/context"
)

// MailCounter is the counter object for the Mail service.
type MailCounter struct {
	Send         Entry
	SendToAdmins Entry
}

type mailCounter struct {
	c *MailCounter

	m mail.Interface
}

var _ mail.Interface = (*mailCounter)(nil)

func (m *mailCounter) Send(msg *mail.Message) error {
	return m.c.Send.up(m.m.Send(msg))
}

func (m *mailCounter) SendToAdmins(msg *mail.Message) error {
	return m.c.SendToAdmins.up(m.m.SendToAdmins(msg))
}

func (m *mailCounter) Testable() mail.Testable {
	return m.m.Testable()
}

// FilterMail installs a counter Mail filter in the context.
func FilterMail(c context.Context) (context.Context, *MailCounter) {
	state := &MailCounter{}
	return mail.AddFilters(c, func(ic context.Context, u mail.Interface) mail.Interface {
		return &mailCounter{state, u}
	}), state
}
