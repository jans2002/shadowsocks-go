package shadowsocks

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
)

type httpRelyParser struct {
	httpVersionString string
	replyStatusCode   int
	replyStatusString string
	headers           map[string]string
	parserStatus      int
	parsingKey        string
	parsingValue      string
	bodyLength        int
}

const (
	parsingHTTPVersionString = 0
	parsingStatusCode        = 1
	parsingStatusString      = 2
	parsingHeadersKey        = 3
	parsingHeadersValue      = 4
)

var (
	httpMethods = map[string]bool{
		"GET":     true,
		"POST":    true,
		"OPTIONS": true,
		"HEAD":    true,
		"PUT":     true,
		"DELETE":  true,
		"CONNECT": true,
		"TRACE":   true,
		"PATCH":   true,
	}
)

func IsValidHTTPMethod(m string) bool {
	_, ok := httpMethods[m]
	return ok
}

func newHTTPReplyParser() *httpRelyParser {
	var parser httpRelyParser
	parser.headers = make(map[string]string)
	return &parser
}

func (parser *httpRelyParser) reset() {
	parser.httpVersionString = ""
	parser.replyStatusString = ""
	parser.parsingKey = ""
	parser.parsingValue = ""
	parser.parserStatus = parsingHTTPVersionString
	parser.bodyLength = 0
	for k := range parser.headers {
		delete(parser.headers, k)
	}
}

func (parser *httpRelyParser) read(b byte) (ok bool, err error) {
	ok = false
	if (b > 126 || b < 32) && b != '\r' && b != '\n' {
		err = fmt.Errorf("Invalid character %u", b)
		return
	}
	switch parser.parserStatus {
	case parsingHTTPVersionString:
		if b == ' ' {
			parser.parserStatus = parsingStatusCode
		} else {
			parser.httpVersionString += string(b)
			if len(parser.httpVersionString) == 4 && parser.httpVersionString != "HTTP" {
				err = fmt.Errorf("Invalid http response")
				return
			}
		}
		break
	case parsingStatusCode:
		if b == ' ' {
			if len(parser.replyStatusString) == 0 {
				err = errors.New("No Reply Status Code")
			} else {
				parser.replyStatusCode, err = strconv.Atoi(parser.replyStatusString)
				if err == nil {
					parser.replyStatusString = ""
					parser.parserStatus = parsingStatusString
				}
			}
		} else if b >= '0' && b <= '9' {
			parser.replyStatusString += string(b)
		} else {
			err = errors.New("Bad Reply Status Code")
		}
		break
	case parsingStatusString:
		if b == '\n' {
			if len(parser.replyStatusString) == 0 {
				err = errors.New("No Reply Status String")
			} else {
				parser.parserStatus = parsingHeadersKey
			}
		} else if b != '\r' {
			parser.replyStatusString += string(b)
		}
		break
	case parsingHeadersKey:
		if b == '\n' {
			parser.parsingKey = ""
			parser.parsingValue = ""
			ok = true
		} else if b != '\r' {
			if b == ':' {
				parser.parserStatus = parsingHeadersValue
			} else {
				if b != ' ' {
					parser.parsingKey += string(b)
				}
			}
		}
		break
	case parsingHeadersValue:
		if b == '\n' {
			parser.parserStatus = parsingHeadersKey
			parser.headers[parser.parsingKey] = parser.parsingValue
			parser.parsingKey = ""
			parser.parsingValue = ""
		} else if b != '\r' {
			if len(parser.parsingValue) != 0 || b != ' ' {
				parser.parsingValue += string(b)
			}
		}
	}
	return
}

func (parser *httpRelyParser) marshal() string {
	var buffer bytes.Buffer
	buffer.WriteString(parser.httpVersionString)
	buffer.WriteString(" ")
	buffer.WriteString(strconv.Itoa(parser.replyStatusCode))
	buffer.WriteString(" ")
	buffer.WriteString(parser.replyStatusString)
	buffer.WriteString("\r\n")
	for k, v := range parser.headers {
		buffer.WriteString(k)
		buffer.WriteString(": ")
		buffer.WriteString(v)
		buffer.WriteString("\r\n")
	}
	buffer.WriteString("\r\n")
	return buffer.String()
}

func (parser *httpRelyParser) getFirstLine() string {
	var buffer bytes.Buffer
	buffer.WriteString(parser.httpVersionString)
	buffer.WriteString(" ")
	buffer.WriteString(strconv.Itoa(parser.replyStatusCode))
	buffer.WriteString(" ")
	buffer.WriteString(parser.replyStatusString)
	return buffer.String()
}

func (parser *httpRelyParser) print() {
	fmt.Print(parser.marshal())
}

// copy from stackoverflow

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func randStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

var requestFormat string
var responseFromat string

func init() {
	var requestBuffer bytes.Buffer
	strs := []string{
		"POST /%s HTTP/1.1\r\n",
		"Accept: */*\r\n",
		"Accept-Encoding: */*\r\n",
		"Accept-Language: zh-CN\r\n",
		"Connection: keep-alive\r\n",
		"%s",
		"Transfer-Encoding: chunked\r\n\r\n",
	}
	for _, str := range strs {
		requestBuffer.WriteString(str)
	}
	requestFormat = requestBuffer.String()
	var responseBuffer bytes.Buffer
	strs = []string{
		"HTTP/1.1 200 OK\r\n",
		"Cache-Control: private, no-store, max-age=0, no-cache\r\n",
		"Content-Type: text/html; charset=utf-8\r\n",
		"Content-Encoding: gzip\r\n",
		"Server: openresty/1.11.2\r\n",
		"Connection: keep-alive\r\n",
		"%s",
		"Transfer-Encoding: chunked\r\n\r\n",
	}
	for _, str := range strs {
		responseBuffer.WriteString(str)
	}
	responseFromat = responseBuffer.String()
}

func buildHTTPRequest(headers string) string {
	return fmt.Sprintf(requestFormat, randStringBytesMaskImprSrc(rand.Intn(48)+1), headers)
}

func buildHTTPResponse(headers string) string {
	return fmt.Sprintf(responseFromat, headers)
}

type httpRequestParser struct {
	httpVersionString string
	requestMethod     string
	requestURI        string
	headers           map[string]string
	parserStatus      int
	parsingKey        string
	parsingValue      string
	bodyLength        int
	originHeader      []byte
}

const (
	parsingMethod      = 0
	parsingURI         = 1
	parsingHTTPVersion = 2
)

func (parser *httpRequestParser) reset() {
	parser.httpVersionString = ""
	parser.requestMethod = ""
	parser.requestURI = ""
	parser.parsingKey = ""
	parser.parsingValue = ""
	parser.parserStatus = parsingMethod
	parser.bodyLength = 0
	for k := range parser.headers {
		delete(parser.headers, k)
	}
}

func (parser *httpRequestParser) read(b byte) (ok bool, err error) {
	ok = false
	if (b > 126 || b < 32) && b != '\r' && b != '\n' {
		err = fmt.Errorf("Invalid character %u", b)
		return
	}
	parser.originHeader = append(parser.originHeader, b)
	switch parser.parserStatus {
	case parsingMethod:
		if b == ' ' {
			if !IsValidHTTPMethod(parser.requestMethod) {
				err = fmt.Errorf("Invalid method")
				return
			}
			parser.parserStatus = parsingURI
		} else {
			if len(parser.requestMethod) > 8 {
				err = fmt.Errorf("Method is too long")
				return
			}
			parser.requestMethod += string(b)
		}
		break
	case parsingURI:
		if b == ' ' {
			if len(parser.requestURI) == 0 {
				err = errors.New("No URI")
			} else {
				parser.parserStatus = parsingHTTPVersion
			}
		} else {
			parser.requestURI += string(b)
		}
		break
	case parsingHTTPVersion:
		if b == '\n' {
			if len(parser.httpVersionString) == 0 {
				err = errors.New("No HTTP Version String")
			} else {
				parser.parserStatus = parsingHeadersKey
			}
		} else if b != '\r' {
			parser.httpVersionString += string(b)
		}
		break
	case parsingHeadersKey:
		if b == '\n' {
			parser.parsingKey = ""
			parser.parsingValue = ""
			ok = true
		} else if b != '\r' {
			if b == ':' {
				parser.parserStatus = parsingHeadersValue
			} else {
				if b != ' ' {
					parser.parsingKey += string(b)
				}
			}
		}
		break
	case parsingHeadersValue:
		if b == '\n' {
			parser.parserStatus = parsingHeadersKey
			parser.headers[parser.parsingKey] = parser.parsingValue
			parser.parsingKey = ""
			parser.parsingValue = ""
		} else if b != '\r' {
			if len(parser.parsingValue) != 0 || b != ' ' {
				parser.parsingValue += string(b)
			}
		}
	}
	return
}

func (parser *httpRequestParser) marshal() string {
	var buffer bytes.Buffer
	buffer.WriteString(parser.requestMethod)
	buffer.WriteString(" ")
	buffer.WriteString(parser.requestURI)
	buffer.WriteString(" ")
	buffer.WriteString(parser.httpVersionString)
	buffer.WriteString("\r\n")
	for k, v := range parser.headers {
		buffer.WriteString(k)
		buffer.WriteString(": ")
		buffer.WriteString(v)
		buffer.WriteString("\r\n")
	}
	buffer.WriteString("\r\n")
	return buffer.String()
}

func (parser *httpRequestParser) getFirstLine() string {
	var buffer bytes.Buffer
	buffer.WriteString(parser.requestMethod)
	buffer.WriteString(" ")
	buffer.WriteString(parser.requestURI)
	buffer.WriteString(" ")
	buffer.WriteString(parser.httpVersionString)
	return buffer.String()
}

func (parser *httpRequestParser) print() {
	fmt.Print(parser.marshal())
}

func newHTTPRequestParser() *httpRequestParser {
	var parser httpRequestParser
	parser.headers = make(map[string]string)
	return &parser
}
