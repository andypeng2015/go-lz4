/*
 * Copyright 2011-2012 Branimir Karadzic. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without modification,
 * are permitted provided that the following conditions are met:
 *
 *    1. Redistributions of source code must retain the above copyright notice, this
 *       list of conditions and the following disclaimer.
 *
 *    2. Redistributions in binary form must reproduce the above copyright notice,
 *       this list of conditions and the following disclaimer in the documentation
 *       and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY COPYRIGHT HOLDER ``AS IS'' AND ANY EXPRESS OR
 * IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT
 * SHALL COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT,
 * INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
 * PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE
 * OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF
 * THE POSSIBILITY OF SUCH DAMAGE.
 */

package lz4

import (
	"errors"
	"io"
)

var (
	ErrCorrupt = errors.New("corrupt input")
)

const (
	mlBits  = 4
	mlMask  = (1 << mlBits) - 1
	runBits = 8 - mlBits
	runMask = (1 << runBits) - 1
)

type decoder struct {
	src  []byte
	dst  []byte
	spos uint32
	dpos uint32
	ref  uint32
}

func (d *decoder) readByte() (uint8, error) {
	if int(d.spos) == len(d.src) {
		return 0, io.EOF
	}
	b := d.src[d.spos]
	d.spos++
	return b, nil
}

func (d *decoder) getLen() (uint32, error) {

	length := uint32(0)
	ln, err := d.readByte()
	if err != nil {
		return 0, ErrCorrupt
	}
	for ln == 255 {
		length += 255
		ln, err = d.readByte()
		if err != nil {
			return 0, ErrCorrupt
		}
	}
	length += uint32(ln)

	return length, nil
}

func (d *decoder) readUint16() (uint16, error) {
	b1, err := d.readByte()
	if err != nil {
		return 0, err
	}
	b2, err := d.readByte()
	if err != nil {
		return 0, ErrCorrupt
	}
	u16 := (uint16(b2) << 8) | uint16(b1)
	return u16, nil
}

func (d *decoder) cp(length, decr uint32) {
	// can't use copy here, but could probably optimize the appends
	if int(d.ref+length) < len(d.dst) {
		d.dst = append(d.dst, d.dst[d.ref:d.ref+length]...)
	} else {
		for ii := uint32(0); ii < length; ii++ {
			d.dst = append(d.dst, d.dst[d.ref+ii])
		}
	}
	d.dpos += length
	d.ref += length - decr
}

func (d *decoder) consume(length uint32) error {

	for ii := uint32(0); ii < length; ii++ {
		by, err := d.readByte()
		if err != nil {
			return ErrCorrupt
		}
		d.dst = append(d.dst, by)
		d.dpos++
	}

	return nil
}

func (d *decoder) finish(err error) error {
	if err == io.EOF {
		return nil
	}

	return err
}

func Decode(dst, src []byte) ([]byte, error) {

	if dst == nil {
		dst = make([]byte, len(src)) // guess
	}

	dst = dst[:0]

	d := decoder{src: src, dst: dst}

	decr := []uint32{0, 3, 2, 3}

	for {
		code, err := d.readByte()
		if err != nil {
			return d.dst, d.finish(err)
		}

		length := uint32(code >> mlBits)
		if length == runMask {
			ln, err := d.getLen()
			if err != nil {
				return nil, ErrCorrupt
			}
			length += ln
		}

		err = d.consume(length)
		if err != nil {
			return nil, ErrCorrupt
		}

		back, err := d.readUint16()
		if err != nil {
			return d.dst, d.finish(err)
		}
		d.ref = d.dpos - uint32(back)

		length = uint32(code & mlMask)
		if length == mlMask {
			ln, err := d.getLen()
			if err != nil {
				return nil, ErrCorrupt
			}
			length += ln
		}

		literal := d.dpos - d.ref
		if literal < 4 {
			d.cp(4, decr[literal])
		} else {
			length += 4
		}

		d.cp(length, 0)
	}
}
