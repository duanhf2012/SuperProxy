/* connect.h
 *
 * Copyright 2015, Chen Wei <weichen302@gmail.com>
 *
 * License GPLv3: GNU GPL version 3
 * This is free software: you are free to change and redistribute it.
 * There is NO WARRANTY, to the extent permitted by law.
 *
 * Please send bug reports and questions to Chen Wei <weichen302@gmail.com>
 */

#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <netdb.h>
#include <unistd.h>
#include <sys/socket.h>

ssize_t readn(int fd, void *usrbuf, size_t n);
ssize_t writen(int fd, void *usrbuf, size_t n);
int tcp_connect(const char *host, const char *serv);
