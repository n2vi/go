// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gob

import (
	"gob";
	"io";
	"os";
	"reflect";
	"sync";
)

import "fmt"	// TODO DELETE

type Encoder struct {
	sync.Mutex;	// each item must be sent atomically
	sent	map[reflect.Type] uint;	// which types we've already sent
	state	*EncState;	// so we can encode integers, strings directly
}

func NewEncoder(w io.Writer) *Encoder {
	enc := new(Encoder);
	enc.sent = make(map[reflect.Type] uint);
	enc.state = new(EncState);
	enc.state.w = w;	// the rest isn't important; all we need is buffer and writer
	return enc;
}

func (enc *Encoder) badType(rt reflect.Type) {
	enc.state.err = os.ErrorString("can't encode type " + rt.String());
}

func (enc *Encoder) sendType(rt reflect.Type) {
	// Drill down to the base type.
	for {
		pt, ok := rt.(*reflect.PtrType);
		if !ok {
			break
		}
		rt = pt.Elem();
	}

	// We only send structs - everything else is basic or an error
	switch t := rt.(type) {
	case *reflect.StructType:
		break;	// we handle these
	case *reflect.ChanType:
		enc.badType(rt);
		return;
	case *reflect.MapType:
		enc.badType(rt);
		return;
	case *reflect.FuncType:
		enc.badType(rt);
		return;
	case *reflect.InterfaceType:
		enc.badType(rt);
		return;
	default:
		return;	// basic, array, etc; not a type to be sent.
	}

	// Have we already sent this type?
	id, alreadySent := enc.sent[rt];
	if alreadySent {
		return
	}

	// Need to send it.
	info := getTypeInfo(rt);
	// Send the pair (-id, type)
	// Id:
	EncodeInt(enc.state, -int64(info.typeId));
	// Type:
	Encode(enc.state.w, info.wire);
	// Remember we've sent this type.
	enc.sent[rt] = id;
	// Now send the inner types
	st := rt.(*reflect.StructType);
	for i := 0; i < st.NumField(); i++ {
		enc.sendType(st.Field(i).Type);
	}
}

func (enc *Encoder) Encode(e interface{}) os.Error {
	rt, indir := indirect(reflect.Typeof(e));

	// Make sure we're single-threaded through here.
	enc.Lock();
	defer enc.Unlock();

	// Make sure the type is known to the other side.
	enc.sendType(rt);
	if enc.state.err != nil {
		return enc.state.err
	}

	// Identify the type of this top-level value.
	EncodeInt(enc.state, int64(enc.sent[rt]));

	// Finally, send the data
	Encode(enc.state.w, e);

	// Release and return.
	return enc.state.err
}
