
CC = gcc
CFLAGS = -Wall -O2
LIBS =
LIBS += -lz

ProgramName= sproxy

# default target
.PHONY : all
all: $(ProgramName)
	@echo all done!

OBJS =
OBJS += http.o
OBJS += connect.o

SPROXY_OBJS = $(OBJS)
SPROXY_OBJS += sproxy.o

http.o: http.c http.h
connect.o: connect.c connect.h
sproxy.o: sproxy.c

$(ProgramName): $(SPROXY_OBJS)
	$(CC) $(CFLAGS) $^ -o $@ $(LIBS)

.PHONY : clean
clean:
	rm -f *.o core a.out $(ProgramName)
