CC = gcc
CFLAGS = -g -Wall -Werror

all: sproxy

sproxy: sproxy.o
	$(CC) $(CFLAGS) sproxy.o -o sproxy

sproxy.o:
	$(CC) $(CFLAGS) -c sproxy.c

clean:
	rm -rf *.o
	rm sproxy
