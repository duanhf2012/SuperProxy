/* http.c
 *
 * a simple http client with gzip
 *
 * Copyright 2015, Chen Wei <weichen302@gmail.com>
 *
 * License GPLv3: GNU GPL version 3
 * This is free software: you are free to change and redistribute it.
 * There is NO WARRANTY, to the extent permitted by law.
 *
 * Please send bug reports and questions to Chen Wei <weichen302@gmail.com>
 */

#include <stdlib.h>
#include <ctype.h>
#include <zlib.h>
#include <assert.h>
#include "connect.h"
#include "http.h"

static char host[MAXURL];
static char arg[MAXURL];
static char request_string[MAXURL + 4096];

static void parse_url(char *url);
static size_t create_req_str(struct request *req);
static struct request *request_new(const char *method, char *arg);
static int request_set_header(struct request *req, const char *name, char *value);
static int request_send(char *method, char *host, char *arg);
static void request_free (struct request *req);

static int parse_header(struct resp_headers *resp, char *head);
static char *read_drain_headers(int sockfd);
static char *response_head_strdup(struct resp_headers *resp, char *name);
static unsigned char *read_response_body(int sockfd);
static unsigned char *read_response_body_gzip(int sockfd);
static int is_gzip(struct resp_headers *resp);
static void response_free(struct resp_headers *resp);


/* return http status code on success, -1 on error */
int http_get(char *url, char **out)
{
    struct resp_headers *header;
    char *hbuf;  // will be freed by response_free
    char *red_url;
    unsigned char *buf;
    int sockfd;
    int status;
    int red_count = 0;

    parse_url(url);
    if ((sockfd = request_send("GET", host, arg)) == -1) {
        fprintf(stderr, "http_get %s: fail send request\n", url);
        return -1;
    }

    if ((hbuf = read_drain_headers(sockfd)) == NULL) {
        fprintf(stderr, "http_get %s: fail to read response header\n", url);
        return -1;
    }

    header = calloc(1, sizeof(struct resp_headers));
    if ((status = parse_header(header, hbuf)) == -1) {
        fprintf(stderr, "http_get %s%s: fail to parse response header\n",
                host, arg);
        return -1;
    }

    while (300 < status && status < 400 && red_count++ < REDIRECT_LIMIT) {
        printf("HTTP %d: redirect to %s%s\n", status, host, arg);
        red_url = response_head_strdup(header, "Location");
        response_free(header);
        if (red_url[0] == '/')
            strncpy(arg, red_url, MAXURL);
        else
            parse_url(red_url);

        free(red_url);
        close(sockfd);
        if ((sockfd = request_send("GET", host, arg)) == -1) {
            fprintf(stderr, "http_get %s: fail send request\n", url);
            return -1;
        }

        if ((hbuf = read_drain_headers(sockfd)) == NULL) {
            fprintf(stderr, "http_get %s: fail to read response header\n", url);
            return -1;
        }

        header = calloc(1, sizeof(struct resp_headers));
        if ((status = parse_header(header, hbuf)) == -1) {
            fprintf(stderr, "http_get %s%s: fail to parse response header\n",
                    host, arg);
            return -1;
        }
    }

    if (is_gzip(header))
        buf = read_response_body_gzip(sockfd);
    else
        buf = read_response_body(sockfd);

    response_free(header);
    close(sockfd);

    if (buf == NULL) {
        fprintf(stderr, "http_get %s: fail to read response body\n", url);
        return -1;
    }

    *out = (char *) buf;
    return status;
}


/* parse url and save to global string buffer host & arg * */
void parse_url(char *url)
{
    char *p;

    p = strstr(url, ":/");
    if (p == NULL) {
        strcpy(host, url);
    } else {
        p += 3;  // skip ://
        strcpy(host, p);
    }

    p = strchr(host, '/');

    if (p == NULL) {
        strcpy(arg, "/");
    } else {
        strcpy(arg, p);
        *p = '\0';  // end host
    }
}


/* Create a new, empty request. Set the request's method and its
   arguments. */
static struct request *request_new(const char *method, char *arg)
{
    struct request *req;

    if ((req = malloc(sizeof(struct request))) == NULL) {
        fprintf(stderr, "debug: request_new, malloc error\n");
        return NULL;
    }

    req->hlimit = 8;
    req->hcount = 0;
    req->method = method;
    req->arg = arg;
    req->headers = calloc(req->hlimit, sizeof(struct request_header));

    return req;
}


/* Release the resources used by REQ. */
static void request_free (struct request *req)
{
    if (req->headers)
        free(req->headers);
    free(req);
}


/* Set the request named NAME to VALUE. */
static int request_set_header(struct request *req, const char *name, char *value)
{
    struct request_header *hdr, *np;
    int i;

    for (i = 0; i < req->hcount; i++) {          // replace exist header
        hdr = &req->headers[i];
        if (strcasecmp(hdr->name, name) == 0) {
            hdr->value = value;
            return 0;
        }
    }

    if (req->hcount >= req->hlimit) {
        req->hlimit *= 2;
        np = realloc(req->headers, req->hlimit * sizeof(*hdr));
        if (np == NULL) {
            fprintf(stderr, "request_set_header: realloc error\n");
            return -1;
        }

        req->headers = np;
    }

    hdr = &req->headers[req->hcount++];
    hdr->name = name;
    hdr->value = value;
    return 0;
}


static size_t create_req_str(struct request *req)
{
    struct request_header *hdr;
    char *p;
    size_t size = 0;
    int i;

    /* GET /urlpath HTTP/1.0 \r\n */
    size += strlen(req->method) + 1;
    size += strlen(req->arg) + 1 + 8 + 1 + 2;
    for (i = 0; i < req->hcount; i++) {
        /* header_name: header_value\r\n */
        hdr = &req->headers[i];
        size += strlen(hdr->name) + 2 + strlen(hdr->value) + 2;
    }

    size += 2;   // "\r\n"

    // add 1 for '\0', otherwise valgrind show Invalid write of size 1
    p = request_string;
    memset(request_string, 0, sizeof(request_string));
    strcat(p, req->method), strcat(p, " ");
    strcat(p, req->arg);
    strcat(p, " HTTP/1.0 \r\n");
    for (i = 0; i < req->hcount; i++) {
        hdr = &req->headers[i];
        strcat(p, hdr->name), strcat(p, ": ");
        strcat(p, hdr->value), strcat(p, "\r\n");
    }

    strcat(p, "\r\n");
    return size;
}

/* open connect and send request, return a socket fd */
static int request_send(char *method, char *host, char *arg)
{
    struct request *req;
    size_t size;
    int sockfd;

    if ((sockfd = tcp_connect(host, "80")) < 0) {
        fprintf(stderr, "http_get: can not create connection to %s\n", host);
        return -1;
    }

    req = request_new(method, arg);
    request_set_header(req, "Host", host);
    request_set_header(req, "Accept-Encoding", "gzip");
    request_set_header(req, "User-Agent", DEFAULT_USER_AGENT);

    size = create_req_str(req);
    if (writen(sockfd, request_string, size) == -1) {
        fprintf(stderr, "http_get: fail send request to %s\n", host);
        request_free(req);
        return -1;
    }

    request_free(req);
    return sockfd;
}


/* read and drain the header from sockfd, return allocated header  */
static char *read_drain_headers(int fd)
{
    char *buf, *p;
    ssize_t npeek, nread;
    size_t i, headlen;
    size_t buflen = PEEK_SIZE;

    buf = NULL;
    while (buflen < MAX_HEADER_SIZE) {
        if (buf != NULL)
            free(buf);

        if ((buf = calloc(1, buflen)) == NULL) {
            fprintf(stderr, "read_drain_headers: calloc error\n");
            return NULL;
        }

        npeek = recv(fd, buf, buflen, MSG_PEEK);
        if (npeek == -1 && errno == EINTR)
            continue;

        /* looking for header terminator \n\r\n */
        p = buf;
        for (i = 0; i < (buflen - 2); i++) {
            if (p[i] == '\n' && p[i + 1] == '\r' && p[i + 2] == '\n') {
                /* read & drain header from sockfd */
                headlen = i + 3;
                if ((nread = readn(fd, buf, headlen)) == -1) {
                    free(buf);
                    return NULL;
                }

                assert(headlen == nread);
                buf[i + 1] = '\0';

                return buf;
            }
        }

        /* hasn't find terminator, double peek size and try again */
        buflen *= 2;
    }

    fprintf(stderr, "read_drain_headers: header too big\n");
    return NULL;
}


/* parse response header, return http status code , -1 on error */
static int parse_header(struct resp_headers *resp, char *head)
{
    char **np;
    char *p, *s;
    char status_buf[STATUS_BUF];
    int status;

    resp->data = head;
    resp->hcount = 0;
    resp->hlimit = 16;
    resp->headers = malloc(resp->hlimit * sizeof(char **));
    memset(resp->headers, 0, resp->hlimit * sizeof(char **));

    resp->headers[resp->hcount++] = head;
    // header end with \r\n\0, the \r in the extra \r\n is overwritten by \0
    p = head;
    for ( ; p[2] != '\0'; p++) {
        if (p[0] != '\r' && p[1] != '\n')
            continue;

        if (resp->hcount >= resp->hlimit) {
            resp->hlimit *= 2;
            np = realloc(resp->headers, resp->hlimit * sizeof(char **));
            if (np == NULL) {
                fprintf(stderr, "parse_header: realloc error\n");
                free(resp->data);
                free(resp->headers);
                return -1;
            }

            resp->headers = np;
        }

        resp->headers[resp->hcount++] = p + 2;
    }

    strncpy(status_buf, head, STATUS_BUF);
    s = strchr(status_buf, ' ');
    s++;
    p = strchr(s, ' ');
    *p = '\0';
    status = atoi(s);

    return status;
}


static void response_free(struct resp_headers *resp)
{
    free(resp->data);
    free(resp->headers);
    free(resp);
}


/* get a malloc copy of header value, NULL if not found */
static char *response_head_strdup(struct resp_headers *resp, char *name)
{
    const char *p;
    const char *next;
    char *h, *value;
    int i;
    size_t buflen;
    size_t namelen = strlen(name);

    p = NULL;
    for (i = 0; i < resp->hcount; i++) {
        if (strncasecmp(name, resp->headers[i], namelen) == 0) {
            p = resp->headers[i];
            break;
        }
    }

    if (p == NULL)
        return NULL;

    if (i == (resp->hcount - 1))
        next = resp->headers[0] + strlen(resp->data);
    else
        next = resp->headers[i + 1]; // - resp->headers[0];

    h = strchr(p, ':'), h++;   // skip :
    while (isspace(*h)) {
        h++;
    }

    buflen = next - h - 2 + 1;   // minus \r\n, plus '\0'
    value = calloc(1, buflen);
    strncpy(value, h, buflen);
    value[buflen - 1] = '\0';

    return value;
}


static unsigned char *read_response_body(int fd)
{
    unsigned char *buf, *np;
    ssize_t nread, ntotal;
    size_t bufsize = HTTP_BUFSIZE;

    buf = calloc(1, bufsize);
    ntotal = 0;
    while (1) {
        if ((nread = readn(fd, buf + ntotal, bufsize - ntotal)) < 0) {
            free(buf);
            return NULL;
        } else if (nread == 0) {
            break;
        }

        ntotal += nread;
        if (ntotal == bufsize) {
            bufsize *= 2;
            np = realloc(buf, bufsize);
            if (np == NULL) {
                fprintf(stderr, "debug: read_response_body, realloc error\n");
                free(buf);
                return NULL;
            }

            buf = np;
            memset(buf + ntotal, 0, bufsize - ntotal);
        }
    }

    return buf;
}


/* Decompress from file descriptor until stream ends or EOF.
 * return NULL if error, malloced gunzipped string on success. */
static unsigned char *read_response_body_gzip(int sockfd)
{
    int ret;
    unsigned char inbuf[CHUNK];
    unsigned char *buf, *np;
    size_t buflen = CHUNK;
    int nwritten_total = 0;
    z_stream strm;
    unsigned int bufleft;

    /* allocate inflate state */
    strm.zalloc = Z_NULL;
    strm.zfree = Z_NULL;
    strm.opaque = Z_NULL;
    strm.avail_in = 0;
    strm.next_in = Z_NULL;
    ret = inflateInit2(&strm, 16+MAX_WBITS);
    if (ret != Z_OK)
        return NULL;

    buf = NULL;
    /* decompress until deflate stream ends or end of file */
    do {
        /* fill & refill inbuf */
        memset(inbuf, 0, CHUNK);
        strm.avail_in = readn(sockfd, inbuf, CHUNK);
        if (strm.avail_in < 0) {
            (void)inflateEnd(&strm);
            fprintf(stderr, "debug: inflate error 1 free buf\n");
            if (buf != NULL)
                free(buf);

            return NULL;
        }

        if (strm.avail_in == 0)
            break;
        strm.next_in = inbuf;

        /* run inflate() on inbuf until output buffer not full */
        do {
            if ((buflen - nwritten_total) < CHUNK)
                buflen += CHUNK;

            strm.avail_out = buflen - nwritten_total;
            bufleft = strm.avail_out;
            np = realloc(buf, buflen);
            if (np == NULL) {
                fprintf(stderr, "debug: inflate realloc error\n");
                free(buf);
                return NULL;
            }

            buf = np;
            memset(buf + nwritten_total, 0, bufleft);
            strm.next_out = buf + nwritten_total;
            ret = inflate(&strm, Z_NO_FLUSH);
            assert(ret != Z_STREAM_ERROR);  /* state not clobbered */
            switch (ret) {
            case Z_NEED_DICT:
                ret = Z_DATA_ERROR;     /* and fall through */
            case Z_DATA_ERROR:
            case Z_MEM_ERROR:
                (void)inflateEnd(&strm);
                fprintf(stderr, "debug: inflate error\n");
                free(buf);
                return NULL;
            }

            nwritten_total += bufleft - strm.avail_out;
        } while (strm.avail_out == 0);

    } while (ret != Z_STREAM_END);

    /* clean up and return */
    (void)inflateEnd(&strm);
    return ret == Z_STREAM_END ? buf : NULL;
}


/* scan for Content-Encoding: gzip, return 1 if gzip, 0 if not */
static int is_gzip(struct resp_headers *resp)
{
    char *enc;
    int rc = 0;

    enc = response_head_strdup(resp, "Content-Encoding");
    if (enc == NULL)
        return 0;

    if (strncasecmp(enc, "gzip", 4) == 0)
        rc = 1;

    free(enc);

    return rc;
}
