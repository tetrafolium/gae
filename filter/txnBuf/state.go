// Copyright 2015 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package txnBuf

import (
	"bytes"
	"sync"

	"github.com/tetrafolium/gae/impl/memory"
	"github.com/tetrafolium/gae/service/datastore"
	"github.com/tetrafolium/gae/service/datastore/serialize"
	"github.com/tetrafolium/gae/service/info"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/parallel"
	"github.com/luci/luci-go/common/stringset"
	"golang.org/x/net/context"
)

// DefaultSizeBudget is the size budget for the root transaction.
//
// Because our estimation algorithm isn't entirely correct, we take 5% off
// the limit for encoding and estimate inaccuracies.
//
// 10MB taken on 2015/09/24:
// https://cloud.google.com/appengine/docs/go/datastore/#Go_Quotas_and_limits
const DefaultSizeBudget = int64((10 * 1000 * 1000) * 0.95)

// DefaultWriteCountBudget is the maximum number of entities that can be written
// in a single call.
//
// This is not known to be documented, and has instead been extracted from a
// datastore error message.
const DefaultWriteCountBudget = 500

// XGTransactionGroupLimit is the number of transaction groups to allow in an
// XG transaction.
//
// 25 taken on 2015/09/24:
// https://cloud.google.com/appengine/docs/go/datastore/transactions#Go_What_can_be_done_in_a_transaction
const XGTransactionGroupLimit = 25

// sizeTracker tracks the size of a buffered transaction. The rules are simple:
//   * deletes count for the size of their key, but 0 data
//   * puts count for the size of their key plus the 'EstimateSize' for their
//     data.
type sizeTracker struct {
	keyToSize map[string]int64
	total     int64
}

// set states that the given key is being set to an entity with the size `val`.
// A val of 0 means "I'm deleting this key"
func (s *sizeTracker) set(key string, val int64) {
	if s.keyToSize == nil {
		s.keyToSize = make(map[string]int64)
	}
	prev, existed := s.keyToSize[key]
	s.keyToSize[key] = val
	s.total += val - prev
	if !existed {
		s.total += int64(len(key))
	}
}

// get returns the currently tracked size for key, and wheter or not the key
// has any tracked value.
func (s *sizeTracker) get(key string) (int64, bool) {
	size, has := s.keyToSize[key]
	return size, has
}

// has returns true iff key has a tracked value.
func (s *sizeTracker) has(key string) bool {
	_, has := s.keyToSize[key]
	return has
}

// numWrites returns the number of tracked write operations.
func (s *sizeTracker) numWrites() int {
	return len(s.keyToSize)
}

// dup returns a duplicate sizeTracker.
func (s *sizeTracker) dup() *sizeTracker {
	if len(s.keyToSize) == 0 {
		return &sizeTracker{}
	}
	k2s := make(map[string]int64, len(s.keyToSize))
	for k, v := range s.keyToSize {
		k2s[k] = v
	}
	return &sizeTracker{k2s, s.total}
}

type txnBufState struct {
	sync.Mutex

	// encoded key -> size of entity. A size of 0 means that the entity is
	// deleted.
	entState *sizeTracker
	bufDS    datastore.RawInterface

	roots     stringset.Set
	rootLimit int

	aid      string
	ns       string
	parentDS datastore.RawInterface

	// sizeBudget is the number of bytes that this transaction has to operate
	// within. It's only used when attempting to apply() the transaction, and
	// it is the threshold for the delta of applying this transaction to the
	// parent transaction. Note that a buffered transaction could actually have
	// a negative delta if the parent transaction had many large entities which
	// the inner transaction deleted.
	sizeBudget int64
	// countBudget is the number of entity writes that this transaction has to
	// operate in.
	writeCountBudget int
}

func withTxnBuf(ctx context.Context, cb func(context.Context) error, opts *datastore.TransactionOptions) error {
	inf := info.Get(ctx)
	ns := inf.GetNamespace()

	parentState, _ := ctx.Value(dsTxnBufParent).(*txnBufState)
	roots := stringset.New(0)
	rootLimit := 1
	if opts != nil && opts.XG {
		rootLimit = XGTransactionGroupLimit
	}
	sizeBudget, writeCountBudget := DefaultSizeBudget, DefaultWriteCountBudget
	if parentState != nil {
		// TODO(riannucci): this is a bit wonky since it means that a child
		// transaction declaring XG=true will only get to modify 25 groups IF
		// they're same groups affected by the parent transactions. So instead of
		// respecting opts.XG for inner transactions, we just dup everything from
		// the parent transaction.
		roots = parentState.roots.Dup()
		rootLimit = parentState.rootLimit

		sizeBudget = parentState.sizeBudget - parentState.entState.total
		writeCountBudget = parentState.writeCountBudget - parentState.entState.numWrites()
	}

	bufDS, err := memory.NewDatastore(inf.FullyQualifiedAppID(), ns)
	if err != nil {
		return err
	}

	state := &txnBufState{
		entState:         &sizeTracker{},
		bufDS:            bufDS.Raw(),
		roots:            roots,
		rootLimit:        rootLimit,
		ns:               ns,
		aid:              inf.AppID(),
		parentDS:         datastore.Get(context.WithValue(ctx, dsTxnBufHaveLock, true)).Raw(),
		sizeBudget:       sizeBudget,
		writeCountBudget: writeCountBudget,
	}
	if err = cb(context.WithValue(ctx, dsTxnBufParent, state)); err != nil {
		return err
	}

	// no reason to unlock this ever. At this point it's toast.
	state.Lock()

	if parentState == nil {
		return commitToReal(state)
	}

	if err = parentState.canApplyLocked(state); err != nil {
		return err
	}

	parentState.commitLocked(state)
	return nil
}

// item is a temporary object for representing key/entity pairs and their cache
// state (e.g. if they exist in the in-memory datastore buffer or not).
// Additionally item memoizes some common comparison strings. item objects
// must never be persisted outside of a single function/query context.
type item struct {
	key      *datastore.Key
	data     datastore.PropertyMap
	buffered bool

	encKey string

	// cmpRow is used to hold the toComparableString value for this item during
	// a query.
	cmpRow string

	// err is a bit of a hack for passing back synchronized errors from
	// queryToIter.
	err error
}

func (i *item) getEncKey() string {
	if i.encKey == "" {
		i.encKey = string(serialize.ToBytes(i.key))
	}
	return i.encKey
}

func (i *item) getCmpRow(lower, upper []byte, order []datastore.IndexColumn) string {
	if i.cmpRow == "" {
		row, key := toComparableString(lower, upper, order, i.key, i.data)
		i.cmpRow = string(row)
		if i.encKey == "" {
			i.encKey = string(key)
		}
	}
	return i.cmpRow
}

func (t *txnBufState) updateRootsLocked(roots stringset.Set) error {
	curRootLen := t.roots.Len()
	proposedRoots := stringset.New(1)
	roots.Iter(func(root string) bool {
		if !t.roots.Has(root) {
			proposedRoots.Add(root)
		}
		return proposedRoots.Len()+curRootLen <= t.rootLimit
	})
	if proposedRoots.Len()+curRootLen > t.rootLimit {
		return ErrTooManyRoots
	}
	// only need to update the roots if they did something that required updating
	if proposedRoots.Len() > 0 {
		proposedRoots.Iter(func(root string) bool {
			t.roots.Add(root)
			return true
		})
	}
	return nil
}

func (t *txnBufState) getMulti(keys []*datastore.Key, metas datastore.MultiMetaGetter, cb datastore.GetMultiCB, haveLock bool) error {
	encKeys, roots := toEncoded(keys)
	data := make([]item, len(keys))

	idxMap := []int(nil)
	toGetKeys := []*datastore.Key(nil)

	lme := errors.NewLazyMultiError(len(keys))
	err := func() error {
		if !haveLock {
			t.Lock()
			defer t.Unlock()
		}

		if err := t.updateRootsLocked(roots); err != nil {
			return err
		}

		for i, key := range keys {
			data[i].key = key
			data[i].encKey = encKeys[i]
			if size, ok := t.entState.get(data[i].getEncKey()); ok {
				data[i].buffered = true
				if size > 0 {
					idxMap = append(idxMap, i)
					toGetKeys = append(toGetKeys, key)
				}
			}
		}

		if len(toGetKeys) > 0 {
			j := 0
			t.bufDS.GetMulti(toGetKeys, nil, func(pm datastore.PropertyMap, err error) error {
				impossible(err)
				data[idxMap[j]].data = pm
				j++
				return nil
			})
		}

		idxMap = nil
		getKeys := []*datastore.Key(nil)
		getMetas := datastore.MultiMetaGetter(nil)

		for i, itm := range data {
			if !itm.buffered {
				idxMap = append(idxMap, i)
				getKeys = append(getKeys, itm.key)
				getMetas = append(getMetas, metas.GetSingle(i))
			}
		}

		if len(idxMap) > 0 {
			j := 0
			err := t.parentDS.GetMulti(getKeys, getMetas, func(pm datastore.PropertyMap, err error) error {
				if err != datastore.ErrNoSuchEntity {
					i := idxMap[j]
					if !lme.Assign(i, err) {
						data[i].data = pm
					}
				}
				j++
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	}()
	if err != nil {
		return err
	}

	for i, itm := range data {
		err := lme.GetOne(i)
		if err != nil {
			cb(nil, err)
		} else if itm.data == nil {
			cb(nil, datastore.ErrNoSuchEntity)
		} else {
			cb(itm.data, nil)
		}
	}
	return nil
}

func (t *txnBufState) deleteMulti(keys []*datastore.Key, cb datastore.DeleteMultiCB, haveLock bool) error {
	encKeys, roots := toEncoded(keys)

	err := func() error {
		if !haveLock {
			t.Lock()
			defer t.Unlock()
		}

		if err := t.updateRootsLocked(roots); err != nil {
			return err
		}

		i := 0
		err := t.bufDS.DeleteMulti(keys, func(err error) error {
			impossible(err)
			t.entState.set(encKeys[i], 0)
			i++
			return nil
		})
		impossible(err)
		return nil
	}()
	if err != nil {
		return err
	}

	for range keys {
		cb(nil)
	}

	return nil
}

func (t *txnBufState) fixKeys(keys []*datastore.Key) ([]*datastore.Key, error) {
	lme := errors.NewLazyMultiError(len(keys))
	realKeys := []*datastore.Key(nil)
	for i, key := range keys {
		if key.Incomplete() {
			// intentionally call AllocateIDs without lock.
			start, err := t.parentDS.AllocateIDs(key, 1)
			if !lme.Assign(i, err) {
				if realKeys == nil {
					realKeys = make([]*datastore.Key, len(keys))
					copy(realKeys, keys)
				}

				aid, ns, toks := key.Split()
				toks[len(toks)-1].IntID = start
				realKeys[i] = datastore.NewKeyToks(aid, ns, toks)
			}
		}
	}
	err := lme.Get()

	if realKeys != nil {
		return realKeys, err
	}
	return keys, err
}

func (t *txnBufState) putMulti(keys []*datastore.Key, vals []datastore.PropertyMap, cb datastore.PutMultiCB, haveLock bool) error {
	keys, err := t.fixKeys(keys)
	if err != nil {
		for _, e := range err.(errors.MultiError) {
			cb(nil, e)
		}
		return nil
	}

	encKeys, roots := toEncoded(keys)

	err = func() error {
		if !haveLock {
			t.Lock()
			defer t.Unlock()
		}

		if err := t.updateRootsLocked(roots); err != nil {
			return err
		}

		i := 0
		err := t.bufDS.PutMulti(keys, vals, func(k *datastore.Key, err error) error {
			impossible(err)
			t.entState.set(encKeys[i], vals[i].EstimateSize())
			i++
			return nil
		})
		impossible(err)
		return nil
	}()
	if err != nil {
		return err
	}

	for _, k := range keys {
		cb(k, nil)
	}
	return nil
}

func commitToReal(s *txnBufState) error {
	toPut, toPutKeys, toDel := s.effect()

	return parallel.FanOutIn(func(ch chan<- func() error) {
		if len(toPut) > 0 {
			ch <- func() error {
				mErr := errors.NewLazyMultiError(len(toPut))
				i := 0
				err := s.parentDS.PutMulti(toPutKeys, toPut, func(_ *datastore.Key, err error) error {
					mErr.Assign(i, err)
					i++
					return nil
				})
				if err == nil {
					err = mErr.Get()
				}
				return err
			}
		}
		if len(toDel) > 0 {
			ch <- func() error {
				mErr := errors.NewLazyMultiError(len(toDel))
				i := 0
				err := s.parentDS.DeleteMulti(toDel, func(err error) error {
					mErr.Assign(i, err)
					i++
					return nil
				})
				if err == nil {
					err = mErr.Get()
				}
				return err
			}
		}
	})
}

func (t *txnBufState) effect() (toPut []datastore.PropertyMap, toPutKeys, toDel []*datastore.Key) {
	// TODO(riannucci): preallocate return slices

	// need to pull all items out of the in-memory datastore. Fortunately we have
	// kindless queries, and we disabled all the special entities, so just
	// run a kindless query without any filters and it will return all data
	// currently in bufDS :).
	fq, err := datastore.NewQuery("").Finalize()
	impossible(err)

	err = t.bufDS.Run(fq, func(key *datastore.Key, data datastore.PropertyMap, _ datastore.CursorCB) error {
		toPutKeys = append(toPutKeys, key)
		toPut = append(toPut, data)
		return nil
	})
	memoryCorruption(err)

	for keyStr, size := range t.entState.keyToSize {
		if size == 0 {
			k, err := serialize.ReadKey(bytes.NewBufferString(keyStr), serialize.WithoutContext, t.aid, t.ns)
			memoryCorruption(err)
			toDel = append(toDel, k)
		}
	}

	return
}

func (t *txnBufState) canApplyLocked(s *txnBufState) error {
	proposedState := t.entState.dup()

	for k, v := range s.entState.keyToSize {
		proposedState.set(k, v)
	}
	switch {
	case proposedState.numWrites() > t.writeCountBudget:
		// The new net number of writes must be below the parent's write count
		// cutoff.
		fallthrough

	case proposedState.total > t.sizeBudget:
		// Make sure our new calculated size is within the parent's size budget.
		//
		// We have:
		// - proposedState.total: The "new world" total bytes were this child
		//   transaction committed to the parent.
		// - t.sizeBudget: The maximum number of bytes that this parent can
		//   accommodate.
		return ErrTransactionTooLarge
	}

	return nil
}

func (t *txnBufState) commitLocked(s *txnBufState) {
	toPut, toPutKeys, toDel := s.effect()

	if len(toPut) > 0 {
		impossible(t.putMulti(toPutKeys, toPut,
			func(_ *datastore.Key, err error) error { return err }, true))
	}

	if len(toDel) > 0 {
		impossible(t.deleteMulti(toDel, func(err error) error { return err }, true))
	}
}

// toEncoded returns a list of all of the serialized versions of these keys,
// plus a stringset of all the encoded root keys that `keys` represents.
func toEncoded(keys []*datastore.Key) (full []string, roots stringset.Set) {
	roots = stringset.New(len(keys))
	full = make([]string, len(keys))
	for i, k := range keys {
		roots.Add(string(serialize.ToBytes(k.Root())))
		full[i] = string(serialize.ToBytes(k))
	}
	return
}
