/* http.h
 *
 * Copyright 2015, Chen Wei <weichen302@gmail.com>
 *
 * License GPLv3: GNU GPL version 3
 * This is free software: you are free to change and redistribute it.
 * There is NO WARRANTY, to the extent permitted by law.
 *
 * Please send bug reports and questions to Chen Wei <weichen302@gmail.com>
 */


#define DEFAULT_USER_AGENT "Mozilla/5.0 (X11; U; Linux i686; en-US; rv:1.9.2.13) Gecko"
#define MAXURL 2048  /* IE's limit is around 204x */
#define PEEK_SIZE 1024
#define MAX_HEADER_SIZE 4 * 1024
#define HTTP_BUFSIZE 16 * 1024
#define CHUNK 16 * 1024  /* zlib inflate buf chunk, can be as small as 1 */
#define STATUS_BUF 32
#define REDIRECT_LIMIT 5

struct request {
    const char *method;
    char *arg;
    struct request_header {
        const char *name;
        char *value;
    } *headers;
    int hcount;
    int hlimit;
};

struct resp_headers {
    char *data;

/* From wget: The array of pointers that indicate where each header starts.
For example, given this HTTP response:

HTTP/1.0 200 Ok
Description: some
text
Etag: x

The headers are located like this:

"HTTP/1.0 200 Ok\r\nDescription: some text\r\nEtag: some text\r\n\r\n"
^                   ^                         ^                  ^
headers[0]          headers[1]                headers[2]         \0 */
    char **headers;
    int hcount;
    int hlimit;
};

int http_get(char *url, char **out);
