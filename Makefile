include $(GOROOT)/src/Make.inc

TARG=gofigure
GOFILES=gofigure.go

compile: all

run: all
	./$(TARG) http://localhost:8080

include $(GOROOT)/src/Make.cmd
