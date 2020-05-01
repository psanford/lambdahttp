package lambdahttpv2

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

type LambdaHandler func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error)

func NewLambdaHandler(h http.Handler) LambdaHandler {
	return func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
		w := newRespsonseWriter()
		r := newRequest(req)

		h.ServeHTTP(w, r)

		return w.Response(), nil
	}
}

func newRespsonseWriter() *ResponseWriter {
	header := make(http.Header)

	return &ResponseWriter{
		header:     header,
		statusCode: http.StatusOK,
	}
}

func newRequest(req events.APIGatewayV2HTTPRequest) *http.Request {
	u := url.URL{
		Host:     req.Headers["Host"],
		Scheme:   req.Headers["X-Forwarded-Proto"],
		Path:     req.RawPath,
		RawQuery: req.RawQueryString,
	}

	var bodyReader io.Reader = bytes.NewBufferString(req.Body)
	if req.IsBase64Encoded {
		bodyReader = base64.NewDecoder(base64.StdEncoding, bodyReader)
	}

	method := req.RequestContext.HTTP.Method

	httpReq, err := http.NewRequest(method, u.String(), bodyReader)
	if err != nil {
		panic(err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	httpReq.RemoteAddr = req.RequestContext.HTTP.SourceIP
	return httpReq
}

type ResponseWriter struct {
	header     http.Header
	b          bytes.Buffer
	statusCode int
}

func (w *ResponseWriter) Header() http.Header {
	return w.header
}

func (w *ResponseWriter) Write(p []byte) (int, error) {
	return w.b.Write(p)
}

func (w *ResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *ResponseWriter) Response() events.APIGatewayProxyResponse {
	headers := make(map[string]string)

	resp := events.APIGatewayProxyResponse{
		StatusCode:        w.statusCode,
		MultiValueHeaders: w.header,
	}

	contentType := headers["Content-Type"]

	isText := strings.HasPrefix(contentType, "text") ||
		strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "application/javascript")

	if isText {
		resp.Body = w.b.String()
	} else {
		resp.IsBase64Encoded = true
		resp.Body = base64.StdEncoding.EncodeToString(w.b.Bytes())
	}

	return resp
}
