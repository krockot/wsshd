# Copyright 2009 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.inc

TARG=wsshd
GOFILES=\
	msg.go \
	wsshd.go
GOFLAGS=-I .

include $(GOROOT)/src/Make.cmd
