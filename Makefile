include $(GOROOT)/src/Make.inc

TARG=gofigure
GOFILES=gofigure.go

compile: all

run: all
	./$(TARG)

include $(GOROOT)/src/Make.cmd
