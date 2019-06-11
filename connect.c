/* connect.c
 *
 * Copyright 2015, Chen Wei <weichen302@gmail.com>
 *
 * License GPLv3: GNU GPL version 3
 * This is free software: you are free to change and redistribute it.
 * There is NO WARRANTY, to the extent permitted by law.
 *
 * Please send bug reports and questions to Chen Wei <weichen302@gmail.com>
 */

#include "connect.h"

/* from UNP, return -1 on error, size readed on success */
ssize_t readn(int fd, void *usrbuf, size_t n)
{
    size_t nleft = n;
    ssize_t nread;
    char *bufp = usrbuf;

    while (nleft > 0) {
        if ((nread = read(fd, bufp, nleft)) < 0) {
            if (errno == EINTR)
                nread = 0;
            else
                return -1;
        } else if (nread == 0)      // EOF
            break;

        nleft -= nread;
        bufp += nread;
    }

    return (n - nleft);
}

/* from UNP */
ssize_t writen(int fd, void *usrbuf, size_t n)
{
    size_t nleft = n;
    ssize_t nwritten;
    char *bufp = usrbuf;

    while (nleft > 0) {
        if ((nwritten = write(fd, bufp, nleft)) <= 0) {
            if (errno == EINTR)
                nwritten = 0;
            else
                return -1;
        }
        nleft -= nwritten;
        bufp += nwritten;
    }

    return n;
}


/* connect to host:serv, return socket file descriptor, -1 on error */
int tcp_connect(const char *host, const char *serv)
{
    int n, sockfd;
    struct addrinfo hints;
    struct addrinfo *res, *rp;

    memset(&hints, 0, sizeof(hints));
    //hints.ai_flags = AI_CANONNAME;
    hints.ai_family = AF_UNSPEC;      /* Allow IPv4 or IPv6 */
    hints.ai_socktype = SOCK_STREAM;

    if ((n = getaddrinfo(host, serv, &hints, &res)) != 0) {
        fprintf(stderr, "getaddrinfo: %s\n", gai_strerror(n));
        return -1;
    }

    for (rp = res; rp != NULL; rp = rp->ai_next) {
        sockfd = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (sockfd == -1)
            continue;

        if (connect(sockfd, rp->ai_addr, rp->ai_addrlen) != -1)
            break;                 /* Success */

        close(sockfd);
    }

    if (rp == NULL) {             /* No address succeeded */
        fprintf(stderr, "Could not connect\n");
        return -1;
    }

    freeaddrinfo(res);
    return sockfd;
}
